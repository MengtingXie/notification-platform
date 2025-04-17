package ioc

import (
	"context"

	prodioc "gitee.com/flycash/notification-platform/internal/ioc"
	"gitee.com/flycash/notification-platform/internal/repository"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/notification/callback"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	quotasvc "gitee.com/flycash/notification-platform/internal/service/quota"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	"github.com/gotomicro/ego/server/egrpc"
	"github.com/gotomicro/ego/task/ecron"
)

type App struct {
	GrpcServer *egrpc.Component
	Tasks      []prodioc.Task
	Crons      []ecron.Ecron

	CallbackSvc     callback.Service
	CallbackLogRepo repository.CallbackLogRepository

	ConfigSvc  configsvc.BusinessConfigService
	ConfigRepo repository.BusinessConfigRepository

	NotificationSvc     notificationsvc.Service
	SendNotificationSvc notificationsvc.SendService
	NotificationRepo    repository.NotificationRepository

	ProviderSvc  providersvc.Service
	ProviderRepo repository.ProviderRepository

	QuotaSvc  quotasvc.Service
	QuotaRepo repository.QuotaRepository

	TemplateSvc  templatesvc.ChannelTemplateService
	TemplateRepo repository.ChannelTemplateRepository

	TxNotificationSvc  notificationsvc.TxNotificationService
	TxNotificationRepo repository.TxNotificationRepository
}

func (a *App) StartTasks(ctx context.Context) {
	for _, t := range a.Tasks {
		go func(t prodioc.Task) {
			t.Start(ctx)
		}(t)
	}
}
