//go:build wireinject

package executor

import (
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/sender"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/strategy"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	"github.com/google/wire"
	"github.com/sony/sonyflake"
)

func InitModule(
	idGenerator *sonyflake.Sonyflake,
	notificationSvc notificationsvc.Service,
	configSvc configsvc.Service,
	providerSvc providersvc.Service,
	templateSvc templatesvc.Service,
	smsClients map[string]sms.Client,
) *Module {
	wire.Build(
		sender.NewSender,
		strategy.NewDispatcher,
		service.NewExecutorService,
		wire.Struct(new(Module), "*"),
	)
	return nil
}
