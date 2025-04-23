package connpool

import (
	"context"
	"database/sql"
	"sync/atomic"
	"time"

	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/event/failover"
	"github.com/gotomicro/ego/core/elog"
)

type DBWithFailOver struct {
	*sql.DB
	logger      *elog.Component
	health      atomic.Bool
	failCounter atomic.Int32 // 连续失败计数器
	succCounter atomic.Int32 // 连续成功计数器（用于恢复）
	provider    failover.Provider
}

func (d *DBWithFailOver) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !d.health.Load() {
		// todo 转异步
		err := d.provider.Produce(ctx, failover.ConnPoolEvent{
			Sql:  query,
			Args: args,
		})
		if err != nil {
			d.logger.Error("发送通用转异步的消息失败", elog.FieldErr(err))
		}
		return nil, errs.ErrToAsync
	}
	return d.DB.ExecContext(ctx, query, args...)
}

func (d *DBWithFailOver) healthCheck(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// 如果超时就返回
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// 执行健康检查
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := d.healthOneLoop(timeoutCtx)
		cancel()
		if err != nil {
			d.logger.Error("ConnPool健康检查失败", elog.FieldErr(err))
		}
	}
	return nil
}

func (d *DBWithFailOver) healthOneLoop(ctx context.Context) error {
	err := d.PingContext(ctx)
	if err != nil {
		// 失败时递增失败计数器，重置成功计数器
		d.succCounter.Store(0)
		if d.failCounter.Add(1) >= 3 {
			d.health.Store(false)
			d.failCounter.Store(0) // 重置计数器
		}
		return err
	}
	// 成功时递增成功计数器，重置失败计数器
	d.failCounter.Store(0)
	if d.succCounter.Add(1) >= 3 {
		d.health.Store(true)
		d.succCounter.Store(0) // 重置计数器
	}
	return nil
}
