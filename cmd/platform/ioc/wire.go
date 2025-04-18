//go:build wireinject

package ioc

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/quota"
	"gitee.com/flycash/notification-platform/internal/service/scheduler"

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
		notificationsvc.NewSendingTimeoutTask,
	)
	txNotificationSvcSet = wire.NewSet(
		notificationsvc.NewTxNotificationService,
		repository.NewTxNotificationRepository,
		dao.NewTxNotificationDAO,
		notificationsvc.NewTxCheckTask,
	)
	senderSvcSet = wire.NewSet(
		newSMSClients,
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
		callback.NewAsyncRequestResultCallbackTask,
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
	schedulerSet = wire.NewSet(scheduler.NewScheduler)
	quotaSvcSet  = wire.NewSet(
		quota.NewService,
		quota.NewQuotaMonthlyResetCron,
		repository.NewQuotaRepository,
		dao.NewQuotaDAO)
)

func newChannel(
	clients map[string]client.Client,
	templateSvc templatesvc.ChannelTemplateService,
) channel.Channel {
	return channel.NewDispatcher(map[domain.Channel]channel.Channel{
		domain.ChannelSMS: channel.NewSMSChannel(newSMSSelectorBuilder(clients, templateSvc)),
	})
}

func newSMSSelectorBuilder(
	clients map[string]client.Client,
	templateSvc templatesvc.ChannelTemplateService,
) *sequential.SelectorBuilder {
	// 构建SMS供应商
	providers := make([]provider.Provider, 0, len(clients))
	for name := range clients {
		providers = append(providers, sms.NewSMSProvider(
			name,
			templateSvc,
			clients[name],
		))
	}
	return sequential.NewSelectorBuilder(providers)
}

func newSMSClients(providerSvc providersvc.Service) map[string]client.Client {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	entities, err := providerSvc.GetByChannel(ctx, domain.ChannelSMS)
	if err != nil {
		panic(err)
	}
	clients := make(map[string]client.Client)
	for i := range entities {
		var cli client.Client
		if entities[i].Name == "aliyun" {
			c, err1 := client.NewAliyunSMS(entities[i].RegionID, entities[i].APIKey, entities[i].APISecret)
			if err1 != nil {
				panic(err1)
			}
			cli = c
			clients[entities[i].Name] = cli
		} else if entities[i].Name == "tencentcloud" {
			c, err1 := client.NewTencentCloudSMS(entities[i].RegionID, entities[i].APIKey, entities[i].APISecret, entities[i].APPID)
			if err1 != nil {
				panic(err1)
			}
			cli = c
			clients[entities[i].Name] = cli
		}
	}
	return clients
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

		// 回调服务
		callbackSvcSet,

		// 提供商服务
		providerSvcSet,

		// 模板服务
		templateSvcSet,

		// 审计服务
		auditsvc.NewService,

		// 事务通知服务
		txNotificationSvcSet,

		// 调度器
		schedulerSet,

		// 额度控制服务
		quotaSvcSet,

		// GRPC服务器
		grpcapi.NewServer,
		ioc.InitGrpc,
		ioc.InitTasks,
		ioc.Crons,
		wire.Struct(new(ioc.App), "*"),
	)

	return new(ioc.App)
}
