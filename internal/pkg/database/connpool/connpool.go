package connpool

import (
	"context"
	"database/sql"

	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/event/failover"
	"gitee.com/flycash/notification-platform/internal/pkg/database/monitor"
	"github.com/gotomicro/ego/core/elog"
	"gorm.io/gorm"
)

type DBWithFailOver struct {
	db        gorm.ConnPool
	logger    *elog.Component
	dbMonitor monitor.DBMonitor
	producer  failover.ConnPoolEventProducer
}

func (d *DBWithFailOver) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return d.db.PrepareContext(ctx, query)
}

func (d *DBWithFailOver) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

func (d *DBWithFailOver) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// checkHealth 检查健康状态并在不健康时发送事件
func (d *DBWithFailOver) checkHealth(ctx context.Context, query string, args ...interface{}) error {
	if !d.dbMonitor.Health() {
		err := d.producer.Produce(ctx, failover.ConnPoolEvent{
			SQL:  query,
			Args: args,
		})
		if err != nil {
			d.logger.Error("发送通用转异步的消息失败", elog.FieldErr(err))
		}
		return errs.ErrToAsync
	}
	return nil
}

func (d *DBWithFailOver) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if err := d.checkHealth(ctx, query, args...); err != nil {
		return nil, err
	}
	return d.db.ExecContext(ctx, query, args...)
}

func NewDBWithFailOver(db *sql.DB,
	dbMonitor monitor.DBMonitor,
	producer failover.ConnPoolEventProducer,
) *DBWithFailOver {
	return &DBWithFailOver{
		db:        db,
		logger:    elog.DefaultLogger,
		dbMonitor: dbMonitor,
		producer:  producer,
	}
}
