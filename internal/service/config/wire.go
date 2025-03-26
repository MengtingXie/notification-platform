//go:build wireinject

package config

import (
	"gitee.com/flycash/notification-platform/internal/service/config/internal/repository"
	dao2 "gitee.com/flycash/notification-platform/internal/service/config/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

func InitService(db *egorm.Component) *Module {
	wire.Build(
		initTables,
		dao2.NewBusinessConfigDAO,
		repository.NewBusinessConfigRepository,
		service.NewBusinessConfigService,
		wire.Struct(new(Module), "*"),
	)
	return new(Module)
}

func initTables(db *egorm.Component) error {
	return dao2.InitTables(db)
}
