//go:build wireinject

package startup

import (
	"gitee.com/flycash/notification-platform/internal/service/config"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/google/wire"
)

func InitModule() *config.Module {
	wire.Build(
		testioc.InitDB,
		config.InitService,
	)
	return nil
}
