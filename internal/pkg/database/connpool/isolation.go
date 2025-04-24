package connpool

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

type IsolationConnPool struct {
	core     gorm.ConnPool
	noneCore gorm.ConnPool
}

func (i *IsolationConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return i.getDB(ctx).PrepareContext(ctx, query)
}

func (i *IsolationConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return i.getDB(ctx).ExecContext(ctx, query, args...)
}

func (i *IsolationConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return i.getDB(ctx).QueryContext(ctx, query, args...)
}

func (i *IsolationConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return i.getDB(ctx).QueryRowContext(ctx, query, args...)
}

func (i *IsolationConnPool) getDB(ctx context.Context) gorm.ConnPool {
	if ctx.Value("Priority") == "high" {
		return i.core
	}
	return i.noneCore
}
