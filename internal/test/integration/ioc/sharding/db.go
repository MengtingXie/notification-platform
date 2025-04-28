package sharding

import (
	"gitee.com/flycash/notification-platform/internal/sharding"
	"gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ecodeclub/ekit/syncx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testTableNum = 2
	testDBNum    = 2
)

func InitNotificationSharding() (notificationStrategy, callbacklogStrategy sharding.ShardingStrategy) {
	return sharding.NewShardingStrategy("notification", "notification", testTableNum, testDBNum), sharding.NewShardingStrategy("notification", "callback_log", testTableNum, testDBNum)
}

func InitTxnSharding() (notificationStrategy, txnStrategy sharding.ShardingStrategy) {
	return sharding.NewShardingStrategy("notification", "notification", testTableNum, testDBNum), sharding.NewShardingStrategy("notification", "tx_notification", testTableNum, testDBNum)
}

func InitDbs() *syncx.Map[string, *gorm.DB] {
	dsn0 := "root:root@tcp(localhost:13316)/notification_0?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=1s&readTimeout=3s&writeTimeout=3s&multiStatements=true"
	ioc.WaitForDBSetup(dsn0)
	db0, err := gorm.Open(mysql.Open(dsn0), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(err)
	}

	dsn1 := "root:root@tcp(localhost:13316)/notification_1?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=1s&readTimeout=3s&writeTimeout=3s&multiStatements=true"
	ioc.WaitForDBSetup(dsn1)
	db1, err := gorm.Open(mysql.Open(dsn1), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(err)
	}

	dbs := &syncx.Map[string, *gorm.DB]{}
	dbs.Store("notification_0", db0)
	dbs.Store("notification_1", db1)
	return dbs
}
