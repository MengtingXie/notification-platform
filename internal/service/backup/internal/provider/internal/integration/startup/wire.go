//go:build wireinject

package startup

import (
	"gitee.com/flycash/notification-platform/internal/service/provider"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitProviderService() provider.Service {
	wire.Build(
		encryptKey,
		provider.InitModule,
		testioc.BaseSet,
		wire.FieldsOf(new(provider.Module), "Svc"))
	return nil
}

func encryptKey() string {
	return "wire1234567890abcdefgh337ywz790"
}
