//go:build wireinject

package notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	dao2 "gitee.com/flycash/notification-platform/internal/service/notification/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/service"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
	"github.com/sony/sonyflake"
)

var notificationServiceProviderSet = wire.NewSet(
	dao.NewNotificationDAO,
	repository.NewNotificationRepository,
	service.NewNotificationService,
)

func InitModule(db *egorm.Component, idGenerator *sonyflake.Sonyflake) Module {
	wire.Build(
		initTables,
		convert,
		notificationServiceProviderSet,
		// 封装统一对象
		wire.Struct(new(Module), "*"))
	return Module{}
}

func convert(svc service.NotificationService) Service {
	return svc.(Service)
}

func initTables(db *egorm.Component) error {
	return dao2.InitTables(db)
}
