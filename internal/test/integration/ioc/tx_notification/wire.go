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

type App struct {
	Svc  notification.TxNotificationService
	Task *notification.TxCheckTask
}

func InitTxNotificationService(configSvc config.BusinessConfigService) *App {
	wire.Build(
		testioc.BaseSet,
		dao.NewTxNotificationDAO,
		dao.NewNotificationDAO,
		initRedisClient,
		repository.NewNotificationRepository,
		repository.NewTxNotificationRepository,
		notification.NewTxNotificationService,
		notification.NewTask,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}

func initRedisClient(rdb redis.Cmdable) dlock.Client {
	return dlockRedis.NewClient(rdb)
}
