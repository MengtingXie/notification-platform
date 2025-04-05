//go:build wireinject

package startup

import (
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitNotificationService() notificationsvc.Service {
	wire.Build(
		testioc.BaseSet,
		notificationsvc.InitModule,
		wire.FieldsOf(new(notificationsvc.Module), "Svc"),
	)
	return nil
}
