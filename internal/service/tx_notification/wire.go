//go:build wireinject

package txnotification

import (
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository"
	dao2 "gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository/dao"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

func InitModule(db *egorm.Component, cache redis.Cmdable, notificationModule notification.Module, configModule config.Module) *Module {
	wire.Build(
		initTables,
		dao2.NewTxNotificationDAO,
		repository.NewTxNotificationRepository,
		InitRetryServiceBuilder,
		wire.FieldsOf(new(notification.Module), "Svc"),
		wire.FieldsOf(new(config.Module), "Svc"),
		InitDlickClient,
		InitService,
		wire.Struct(new(Module), "*"),
	)
	return new(Module)
}

func initTables(db *egorm.Component) error {
	return dao2.InitTables(db)
}
