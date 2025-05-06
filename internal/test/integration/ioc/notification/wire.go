//go:build wireinject

package notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

type Service struct {
	Svc             notification.Service
	QuotaCache      cache.QuotaCache
	Repo            repository.NotificationRepository
	QuotaRepo       repository.QuotaRepository
	CallbackLogRepo repository.CallbackLogRepository
}

func Init() *Service {
	wire.Build(
		testioc.BaseSet,
		redis.NewQuotaCache,
		repository.NewNotificationRepository,
		notification.NewNotificationService,
		dao.NewNotificationDAO,

		repository.NewQuotaRepositoryV2,

		repository.NewCallbackLogRepository,
		dao.NewCallbackLogDAO,

		wire.Struct(new(Service), "*"),
	)
	return nil
}
