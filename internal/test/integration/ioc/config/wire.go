//go:build wireinject

package config

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/cache/local"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
	ca "github.com/patrickmn/go-cache"
)

func InitConfigService(localCache *ca.Cache ) *config.BusinessConfigServiceV1 {
	wire.Build(
		testioc.BaseSet,
		local.NewLocalCache,
		redis.NewCache,
		dao.NewBusinessConfigDAO,
		repository.NewBusinessConfigRepository,
		config.NewBusinessConfigService,
	)
	return new(config.BusinessConfigServiceV1)
}