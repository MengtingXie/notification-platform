package monitor

import (
	"context"
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/gotomicro/ego/core/elog"
)

const (
	timeout             = 5 * time.Second
	defaultFailCount    = 3
	defaultSuccessCount = 3
)

//go:generate mockgen -source=./monitor.go -package=monitormocks -destination=./mocks/monitor.mock.go DBMonitor
type DBMonitor interface {
	Health() bool
	// Report 上报数据库调用时的error,来收集调用时的报错。
	Report(err error)
}

func NewHeartbeatDBMonitor(db *sql.DB) *Heartbeat {
	he := &atomic.Bool{}
	he.Store(true)

	h := &Heartbeat{
		db:          db,
		logger:      elog.DefaultLogger,
		health:      he,
		failCounter: &atomic.Int32{},
		succCounter: &atomic.Int32{},
	}
	go h.healthCheck(context.Background())
	return h
}

// Heartbeat 心跳监控
type Heartbeat struct {
	db          *sql.DB
	logger      *elog.Component
	health      *atomic.Bool
	failCounter *atomic.Int32 // 连续失败计数器
	succCounter *atomic.Int32 // 连续成功计数器（用于恢复）
}

func (h *Heartbeat) Health() bool {
	return h.health.Load()
}

func (*Heartbeat) Report(error) {}

func (h *Heartbeat) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// 如果超时就返回
		if ctx.Err() != nil {
			h.logger.Error("ctx超时退出", elog.FieldErr(ctx.Err()))
			return
		}
		// 执行健康检查
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		err := h.healthOneLoop(timeoutCtx)
		cancel()
		if err != nil {
			h.logger.Error("ConnPool健康检查失败", elog.FieldErr(err))
		}
	}
}

func (h *Heartbeat) healthOneLoop(ctx context.Context) error {
	err := h.db.PingContext(ctx)
	if err != nil {
		// 失败时递增失败计数器，重置成功计数器
		h.succCounter.Store(0)
		if h.failCounter.Add(1) >= defaultFailCount {
			h.health.Store(false)
			h.failCounter.Store(0) // 重置计数器
		}
		return err
	}
	// 成功时递增成功计数器，重置失败计数器
	h.failCounter.Store(0)
	if h.succCounter.Add(1) >= defaultSuccessCount {
		h.health.Store(true)
		h.succCounter.Store(0) // 重置计数器
	}
	return nil
}
