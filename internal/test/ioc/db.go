package ioc

import (
	"context"
	"database/sql"
	"time"

	"github.com/ego-component/egorm"
	"github.com/gotomicro/ego/core/econf"

	"github.com/ecodeclub/ekit/retry"
)

func WaitForDBSetup(dsn string) {
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	const maxInterval = 10 * time.Second
	const maxRetries = 10
	strategy, err := retry.NewExponentialBackoffRetryStrategy(time.Second, maxInterval, maxRetries)
	if err != nil {
		panic(err)
	}

	const timeout = 5 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = sqlDB.PingContext(ctx)
		cancel()
		if err == nil {
			break
		}
		next, ok := strategy.Next()
		if !ok {
			panic("WaitForDBSetup 重试失败......")
		}
		time.Sleep(next)
	}
}

var db *egorm.Component

func InitDB() *egorm.Component {
	if db != nil {
		return db
	}
	econf.Set("mysql", map[string]any{
		"dsn":   "root:root@tcp(localhost:13316)/notification?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=1s&readTimeout=3s&writeTimeout=3s&multiStatements=true",
		"debug": true,
	})
	WaitForDBSetup(econf.GetStringMapString("mysql")["dsn"])
	db = egorm.Load("mysql").Build()
	return db
}
