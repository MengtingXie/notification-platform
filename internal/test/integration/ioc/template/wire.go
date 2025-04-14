//go:build wireinject

package template

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func Init(providerSvc providersvc.Service, auditSvc auditsvc.Service) templatesvc.ChannelTemplateService {
	wire.Build(
		testioc.BaseSet,
		repository.NewChannelTemplateRepository,
		templatesvc.NewChannelTemplateService,
		dao.NewChannelTemplateDAO,
	)
	return nil
}
