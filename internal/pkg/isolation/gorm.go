package isolation

import (
	"context"
	"database/sql"

	"github.com/gotomicro/ego/core/elog"
	"gorm.io/gorm"
)

type ctxType string

const (
	Core ctxType = "core"
)

type DB struct {
	coreDB    gorm.ConnPool
	nonCoreDB gorm.ConnPool
	logger    *elog.Component
}

func NewIsolationDB(coreDB, nonCoreDB gorm.ConnPool) *DB {
	return &DB{coreDB: coreDB, nonCoreDB: nonCoreDB, logger: elog.DefaultLogger}
}

func (d *DB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return d.getDB(ctx).PrepareContext(ctx, query)
}

func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.getDB(ctx).ExecContext(ctx, query, args...)
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.getDB(ctx).QueryContext(ctx, query, args...)
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.getDB(ctx).QueryRowContext(ctx, query, args...)
}

func (d *DB) getDB(ctx context.Context) gorm.ConnPool {
	if d.isCore(ctx) {
		return d.coreDB
	}
	return d.nonCoreDB
}

func (d *DB) isCore(ctx context.Context) bool {
	v := ctx.Value(Core)
	return v != nil
}

func WithCore(ctx context.Context) context.Context {
	return context.WithValue(ctx, Core, true)
}
