//go:build wireinject

package config

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

func InitService(db *egorm.Component) *Module {
	wire.Build(
		initTables,
		dao.NewBusinessConfigDAO,
		repository.NewBusinessConfigRepository,
		service.NewBusinessConfigService,
		wire.Struct(new(Module), "*"),
	)
	return new(Module)
}

func initTables(db *egorm.Component) error {
	return dao.InitTables(db)
}
