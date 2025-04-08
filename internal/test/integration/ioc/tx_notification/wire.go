//go:build wireinject

package tx_notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
	"github.com/meoying/dlock-go"
	dlockRedis "github.com/meoying/dlock-go/redis"
	"github.com/redis/go-redis/v9"
)

func InitTxNotificationService(configSvc config.BusinessConfigService) *notification.TxNotificationServiceV1 {
	wire.Build(
		testioc.BaseSet,
		dao.NewTxNotificationDAO,
		initRedisClient,
		repository.NewTxNotificationRepository,
		notification.NewTxNotificationService,
	)
	return &notification.TxNotificationServiceV1{}
}

func initRedisClient(rdb redis.Cmdable) dlock.Client {
	return dlockRedis.NewClient(rdb)
}