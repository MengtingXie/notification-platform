//go:build wireinject

package ioc

import (
	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	"gitee.com/flycash/notification-platform/internal/service/audit"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/executor"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/tx_notification"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/template"
	"github.com/google/wire"
)

var BaseSet = wire.NewSet(InitDB, InitRedis, InitEtcdClient, InitIDGenerator)

func InitGrpcServer() *App {
	wire.Build(
		// 基础设施
		BaseSet,

		// --- 服务构建 ---

		// 配置服务
		config.InitService,
		wire.FieldsOf(new(*config.Module), "Svc"),

		// 通知服务
		notification.InitModule,
		wire.FieldsOf(new(notification.Module), "Svc"),

		// 加密密钥
		InitProviderEncryptKey,

		// 提供商服务
		providersvc.InitModule,
		wire.FieldsOf(new(providersvc.Module), "Svc"),

		// 模板服务
		template.InitModule,
		wire.FieldsOf(new(template.Module), "Svc"),

		// 审计服务
		audit.InitMoudle,
		wire.FieldsOf(new(*audit.Module), "Svc"),

		// SMS客户端初始化
		InitSmsClients,

		// 事务通知服务
		txnotification.InitModule,
		wire.FieldsOf(new(*txnotification.Module), "Svc"),

		// 执行器服务 - 使用原生的InitModule
		executor.InitModule,
		wire.FieldsOf(new(*executor.Module), "Svc"),

		// GRPC服务器
		grpcapi.NewServer,
		InitGrpc,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}
