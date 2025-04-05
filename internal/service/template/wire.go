//go:build wireinject

package template

import (
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/service"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dao.NewChannelTemplateDAO,
	repository.NewChannelTemplateRepository,
	service.NewChannelTemplateService,
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

func convert(svc service.ChannelTemplateService) Service {
	return svc.(Service)
}

func initTables(db *egorm.Component) error {
	return dao.InitTables(db)
}
