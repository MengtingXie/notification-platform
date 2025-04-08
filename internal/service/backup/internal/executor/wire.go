//go:build wireinject

package executor

import (
	templatesvc "gitee.com/flycash/notification-platform/internal/service/backup/internal/template"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/notification/strategy"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"gitee.com/flycash/notification-platform/internal/service/sender"
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
		notificationsvc.NewExecutorService,
		wire.Struct(new(Module), "*"),
	)
	return nil
}
