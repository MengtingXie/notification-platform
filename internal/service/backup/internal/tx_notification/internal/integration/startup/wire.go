//go:build wireinject

package startup

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	dao2 "gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/tx_notification"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitTxNotificationService(notificationModule notification.Module, configModule config.Module) *notification.TxNotificationServiceV1 {
	wire.Build(
		testioc.BaseSet,
		dao2.NewTxNotificationDAO,
		repository.NewTxNotificationRepository,
		txnotification.InitRetryServiceBuilder,
		wire.FieldsOf(new(notification.Module), "Svc"),
		wire.FieldsOf(new(config.Module), "Svc"),
		txnotification.InitDlickClient,
		notification.NewTxNotificationService,
	)
	return &notification.TxNotificationServiceV1{}
}
