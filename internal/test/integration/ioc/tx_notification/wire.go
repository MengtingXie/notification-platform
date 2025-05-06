//go:build wireinject

package tx_notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

type App struct {
	Svc  notification.TxNotificationService
	Task *notification.TxCheckTask
}

func InitTxNotificationService(configSvc config.BusinessConfigService, sender sender.NotificationSender) *App {
	wire.Build(
		testioc.BaseSet,
		dao.NewTxNotificationDAO,
		dao.NewNotificationDAO,
		redis.NewQuotaCache,
		repository.NewNotificationRepository,
		repository.NewTxNotificationRepository,
		notification.NewTxNotificationService,
		notification.NewTask,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}
