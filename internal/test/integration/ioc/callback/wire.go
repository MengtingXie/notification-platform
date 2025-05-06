//go:build wireinject

package callback

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	callbacksvc "gitee.com/flycash/notification-platform/internal/service/notification/callback"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

type Service struct {
	Svc              callbacksvc.Service
	Repo             repository.CallbackLogRepository
	NotificationRepo repository.NotificationRepository
	QuotaRepo        repository.QuotaRepository
}

func Init(cnfigSvc configsvc.BusinessConfigService) *Service {
	wire.Build(
		testioc.BaseSet,
		callbacksvc.NewService,
		repository.NewCallbackLogRepository,
		dao.NewCallbackLogDAO,
		repository.NewNotificationRepository,
		dao.NewNotificationDAO,
		redis.NewQuotaCache,
		repository.NewQuotaRepository,
		dao.NewQuotaDAO,

		wire.Struct(new(Service), "*"),
	)
	return nil
}
