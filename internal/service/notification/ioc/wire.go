//go:build wireinject

package ioc

import (
	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/repository"
	"gitee.com/flycash/notification-platform/internal/service/notification/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/notification/service"
	"github.com/google/wire"
	"gorm.io/gorm"
)

var notificationServiceProviderSet = wire.NewSet(
	dao.NewNotificationDAO,
	repository.NewNotificationRepository,
	service.NewNotificationService,
)

type NotificationService struct {
	ignoredInitTablesErr error // 必须放在第一个
	Svc                  service.NotificationService
}

type Notification = domain.Notification

func InitService(db *gorm.DB) NotificationService {
	wire.Build(
		initTables,
		notificationServiceProviderSet,
		// 封装统一对象
		wire.Struct(new(NotificationService), "*"))
	return NotificationService{}
}

func initTables(db *gorm.DB) error {
	return dao.InitTables(db)
}
