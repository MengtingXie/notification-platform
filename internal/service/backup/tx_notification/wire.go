//go:build wireinject

package txnotification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	dao2 "gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository/dao"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

func InitModule(db *egorm.Component, cache redis.Cmdable, notificationModule notification.Service, configModule config.Service) *Module {
	wire.Build(
		initTables,
		dao.NewTxNotificationDAO,
		repository.NewTxNotificationRepository,
		InitRetryServiceBuilder,
		InitDlickClient,
		InitService,
		wire.Struct(new(Module), "*"),
	)
	return new(Module)
}

func initTables(db *egorm.Component) error {
	return dao2.InitTables(db)
}
