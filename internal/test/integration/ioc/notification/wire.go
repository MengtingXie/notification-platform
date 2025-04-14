//go:build wireinject

package notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

type Service struct {
	Svc             notification.Service
	Repo            repository.NotificationRepository
	QuotaRepo       repository.QuotaRepository
	CallbackLogRepo repository.CallbackLogRepository
}

func Init() *Service {
	wire.Build(
		testioc.BaseSet,
		repository.NewNotificationRepository,
		notification.NewNotificationService,
		dao.NewNotificationDAO,

		repository.NewQuotaRepository,
		dao.NewQuotaDAO,

		repository.NewCallbackLogRepository,
		dao.NewCallbackLogDAO,

		wire.Struct(new(Service), "*"),
	)
	return nil
}
