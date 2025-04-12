//go:build wireinject

package ioc

import (
	"context"
	"time"

	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/ioc"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/cache/local"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	auditsvc "gitee.com/flycash/notification-platform/internal/service/audit"
	"gitee.com/flycash/notification-platform/internal/service/channel"
	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/notification/callback"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"gitee.com/flycash/notification-platform/internal/service/provider/sequential"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"gitee.com/flycash/notification-platform/internal/service/sendstrategy"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	"github.com/google/wire"
)

var (
	BaseSet = wire.NewSet(
		ioc.InitDB,
		ioc.InitDistributedLock,
		ioc.InitEtcdClient,
		ioc.InitIDGenerator,
		ioc.InitRedisClient,
		ioc.InitSMSClients,
		ioc.InitGoCache,

		local.NewLocalCache,
		redis.NewCache,
	)
	configSvcSet = wire.NewSet(
		configsvc.NewBusinessConfigService,
		repository.NewBusinessConfigRepository,
		dao.NewBusinessConfigDAO)
	notificationSvcSet = wire.NewSet(
		notificationsvc.NewNotificationService,
		repository.NewNotificationRepository,
		dao.NewNotificationDAO,
	)
	txNotificationSvcSet = wire.NewSet(
		notificationsvc.NewTxNotificationService,
		repository.NewTxNotificationRepository,
		dao.NewTxNotificationDAO,
	)
	senderSvcSet = wire.NewSet(
		newChannel,
		sender.NewSender,
	)
	sendNotificationSvcSet = wire.NewSet(
		notificationsvc.NewSendService,
		sendstrategy.NewDispatcher,
		sendstrategy.NewImmediateStrategy,
		sendstrategy.NewDefaultStrategy,
	)
	callbackSvcSet = wire.NewSet(
		callback.NewService,
		repository.NewCallbackLogRepository,
		dao.NewCallbackLogDAO,
	)
	providerSvcSet = wire.NewSet(
		providersvc.NewProviderService,
		repository.NewProviderRepository,
		dao.NewProviderDAO,
		// 加密密钥
		ioc.InitProviderEncryptKey,
	)
	templateSvcSet = wire.NewSet(
		templatesvc.NewChannelTemplateService,
		repository.NewChannelTemplateRepository,
		dao.NewChannelTemplateDAO,
	)
)

func newChannel(
	providerSvc providersvc.Service,
	templateSvc templatesvc.ChannelTemplateService,
	clients map[string]client.Client,
) channel.Channel {
	return channel.NewDispatcher(map[domain.Channel]channel.Channel{
		domain.ChannelEmail: channel.NewSMSChannel(newSMSSelectorBuilder(providerSvc, templateSvc, clients)),
	})
}

func newSMSSelectorBuilder(
	providerSvc providersvc.Service,
	templateSvc templatesvc.ChannelTemplateService,
	clients map[string]client.Client,
) *sequential.SelectorBuilder {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	entities, err := providerSvc.GetProvidersByChannel(ctx, domain.ChannelSMS)
	if err != nil {
		panic(err)
	}
	// 构建SMS供应商
	providers := make([]provider.Provider, 0, len(entities))
	for i := range entities {
		providers = append(providers, sms.NewSMSProvider(
			entities[i].Name,
			templateSvc,
			clients[entities[i].Name],
		))
	}
	return sequential.NewSelectorBuilder(providers)
}

func InitGrpcServer() *ioc.App {
	wire.Build(
		// 基础设施
		BaseSet,

		// --- 服务构建 ---

		// 配置服务
		configSvcSet,

		// 通知服务
		notificationSvcSet,
		sendNotificationSvcSet,
		senderSvcSet,
		callbackSvcSet,

		// 提供商服务
		providerSvcSet,

		// 模板服务
		templateSvcSet,

		// 审计服务
		auditsvc.NewService,

		// 事务通知服务
		txNotificationSvcSet,

		// 执行器服务 - 使用原生的InitModule

		// GRPC服务器
		grpcapi.NewServer,
		ioc.InitGrpc,
		wire.Struct(new(ioc.App), "*"),
	)

	return new(ioc.App)
}
