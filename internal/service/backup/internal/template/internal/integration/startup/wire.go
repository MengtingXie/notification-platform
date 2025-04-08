//go:build wireinject

package startup

import (
	auditsvc "gitee.com/flycash/notification-platform/internal/service/backup/internal/audit"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/template"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitChannelTemplateService(providerSvc providersvc.Service, auditSvc auditsvc.Service) template.Service {
	wire.Build(
		testioc.BaseSet,
		template.InitModule,
		wire.FieldsOf(new(template.Module), "Svc"),
	)
	return nil
}
