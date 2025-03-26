//go:build wireinject

package config

import (
	"gitee.com/flycash/notification-platform/internal/service/config/internal/repository"
	dao2 "gitee.com/flycash/notification-platform/internal/service/config/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

func InitModule() *Module {
	wire.Build(
		testioc.InitDB,
		InitBusinessConfigDAO,
		repository.NewBusinessConfigRepository,
		service.NewBusinessConfigService,
		wire.Struct(new(Module), "*"),
	)
	return new(Module)
}

func InitBusinessConfigDAO(db *egorm.Component) dao2.BusinessConfigDAO {
	err := dao2.InitTables(db)
	if err != nil {
		panic(err)
	}
	return dao2.NewBusinessConfigDAO(db)
}
