//go:build wireinject

package startup

import (
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	"gitee.com/flycash/notification-platform/internal/service/backup/executor"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitService(
	notificationSvc notificationsvc.Service,
	configSvc configsvc.Service,
	providerSvc providersvc.Service,
	templateSvc templatesvc.Service,
	smsClients map[string]sms.Client,
) executor.ExecutorService {
	wire.Build(
		testioc.BaseSet,
		executor.InitModule,
		wire.FieldsOf(new(*executor.Module), "Svc"),
	)
	return nil
}
