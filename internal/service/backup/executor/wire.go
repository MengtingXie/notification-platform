//go:build wireinject

package executor

import (
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"gitee.com/flycash/notification-platform/internal/service/strategy"
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
		NewExecutorService,
		wire.Struct(new(Module), "*"),
	)
	return nil
}
