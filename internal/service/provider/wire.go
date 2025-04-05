//go:build wireinject

package provider

import (
	"gitee.com/flycash/notification-platform/internal/service/provider/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/provider/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/provider/internal/service"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

var providerSet = wire.NewSet(
	dao.NewProviderDAO,
	repository.NewProviderRepository,
	service.NewProviderService,
)

func InitModule(db *egorm.Component, encrytKey string) Module {
	wire.Build(
		initTables,
		convert,
		providerSet,
		// 封装统一对象
		wire.Struct(new(Module), "*"))
	return Module{}
}

func convert(svc service.ProviderService) Service {
	return svc.(Service)
}

func initTables(db *egorm.Component) error {
	return dao.InitTables(db)
}
