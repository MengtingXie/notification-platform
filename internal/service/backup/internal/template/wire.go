//go:build wireinject

package template

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	dao2 "gitee.com/flycash/notification-platform/internal/repository/dao"
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/template/internal/repository/dao"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	template2 "gitee.com/flycash/notification-platform/internal/service/template"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dao2.NewChannelTemplateDAO,
	repository.NewChannelTemplateRepository,
	template2.NewChannelTemplateService,
)

func InitModule(db *egorm.Component, providerSvc providersvc.Service, auditsvc auditsvc.Service) Module {
	wire.Build(
		initTables,
		convert,
		ProviderSet,
		// 封装统一对象
		wire.Struct(new(Module), "*"))
	return Module{}
}

func convert(svc template2.ChannelTemplateService) Service {
	return svc.(Service)
}

func initTables(db *egorm.Component) error {
	return dao.InitTables(db)
}
