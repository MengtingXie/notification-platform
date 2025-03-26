//go:build wireinject

package startup

import (
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/config/repository"
	"gitee.com/flycash/notification-platform/internal/service/config/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config/service"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
)

func InitService() *config.Module {
	wire.Build(
		testioc.InitDB,
		InitBusinessConfigDAO,
		repository.NewBusinessConfigRepository,
		service.NewBusinessConfigService,
		wire.Struct(new(config.Module), "*"),
	)
	return new(config.Module)
}

func InitBusinessConfigDAO(db *egorm.Component) dao.BusinessConfigDAO {
	err := dao.InitTables(db)
	if err != nil {
		panic(err)
	}
	return dao.NewBusinessConfigDAO(db)
}
