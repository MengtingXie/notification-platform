//go:build wireinject

package startup

import (
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitChannelTemplateService(providerSvc providersvc.Service, auditSvc auditsvc.Service) templatesvc.Service {
	wire.Build(
		testioc.BaseSet,
		templatesvc.InitModule,
		wire.FieldsOf(new(templatesvc.Module), "Svc"),
	)
	return nil
}
