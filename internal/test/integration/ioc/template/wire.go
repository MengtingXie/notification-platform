//go:build wireinject

package template

import (
	auditevt "gitee.com/flycash/notification-platform/internal/event/audit"
	templateevt "gitee.com/flycash/notification-platform/internal/event/template"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

type Service struct {
	Svc                 templatesvc.ChannelTemplateService
	Repo                repository.ChannelTemplateRepository
	AuditResultConsumer *templateevt.AuditResultConsumer
	AuditResultProducer auditevt.ResultCallbackEventProducer
}

func Init(
	providerSvc providersvc.Service,
	auditSvc auditsvc.Service,
	clients map[string]client.Client,
) (*Service, error) {
	wire.Build(
		testioc.BaseSet,
		templatesvc.NewChannelTemplateService,
		repository.NewChannelTemplateRepository,
		dao.NewChannelTemplateDAO,

		templateevt.NewAuditResultConsumer,

		auditevt.NewResultCallbackEventProducer,

		wire.Struct(new(Service), "*"),
	)
	return nil, nil
}
