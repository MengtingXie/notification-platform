//go:build wireinject

package provider

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func Init() providersvc.Service {
	wire.Build(
		testioc.BaseSet,
		repository.NewProviderRepository,
		providersvc.NewProviderService,
		dao.NewProviderDAO,
	)
	return nil
}
