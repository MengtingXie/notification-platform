//go:build wireinject

package startup

import (
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	executorsvc "gitee.com/flycash/notification-platform/internal/service/executor"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/pkg/client/sms"
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
) service.ExecutorService {
	wire.Build(
		testioc.BaseSet,
		executorsvc.InitModule,
		wire.FieldsOf(new(*executorsvc.Module), "Svc"),
	)
	return nil
}
