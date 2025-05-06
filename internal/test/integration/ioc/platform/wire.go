//go:build wireinject

package ioc

import (
	"time"

	"gitee.com/flycash/notification-platform/internal/service/quota"
	"gitee.com/flycash/notification-platform/internal/service/scheduler"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ecodeclub/ekit/pool"
	"github.com/gotomicro/ego/core/econf"

	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	"gitee.com/flycash/notification-platform/internal/domain"
	prodioc "gitee.com/flycash/notification-platform/internal/ioc"
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
		prodioc.InitDB,
		prodioc.InitDistributedLock,
		prodioc.InitEtcdClient,
		prodioc.InitIDGenerator,
		prodioc.InitRedisClient,
		prodioc.InitGoCache,
		prodioc.InitRedisCmd,

		local.NewLocalCache,
		redis.NewCache,
	)
	configSvcSet = wire.NewSet(
		configsvc.NewBusinessConfigService,
		repository.NewBusinessConfigRepository,
		dao.NewBusinessConfigDAO)
	notificationSvcSet = wire.NewSet(
		redis.NewQuotaCache,
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
		newChannel,
		newTaskPool,
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
		prodioc.InitProviderEncryptKey,
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

func newTaskPool() pool.TaskPool {
	type Config struct {
		InitGo           int           `yaml:"initGo"`
		CoreGo           int32         `yaml:"coreGo"`
		MaxGo            int32         `yaml:"maxGo"`
		MaxIdleTime      time.Duration `yaml:"maxIdleTime"`
		QueueSize        int           `yaml:"queueSize"`
		QueueBacklogRate float64       `yaml:"queueBacklogRate"`
	}
	var cfg Config
	if err := econf.UnmarshalKey("pool", &cfg); err != nil {
		panic(err)
	}
	p, err := pool.NewOnDemandBlockTaskPool(cfg.InitGo, cfg.QueueSize,
		pool.WithQueueBacklogRate(cfg.QueueBacklogRate),
		pool.WithMaxIdleTime(cfg.MaxIdleTime),
		pool.WithCoreGo(cfg.CoreGo),
		pool.WithMaxGo(cfg.MaxGo))
	if err != nil {
		panic(err)
	}
	err = p.Start()
	if err != nil {
		panic(err)
	}
	return p
}

func newChannel(
	templateSvc templatesvc.ChannelTemplateService,
	clients map[string]client.Client,
) channel.Channel {
	return channel.NewDispatcher(map[domain.Channel]channel.Channel{
		domain.ChannelSMS: channel.NewSMSChannel(newSMSSelectorBuilder(templateSvc, clients)),
	})
}

func newSMSSelectorBuilder(
	templateSvc templatesvc.ChannelTemplateService,
	clients map[string]client.Client,
) *sequential.SelectorBuilder {
	// 构建SMS供应商
	providers := make([]provider.Provider, 0, len(clients))
	for k := range clients {
		providers = append(providers, sms.NewSMSProvider(
			k,
			templateSvc,
			clients[k],
		))
	}
	return sequential.NewSelectorBuilder(providers)
}

func InitGrpcServer(clients map[string]client.Client) *testioc.App {
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
		prodioc.InitGrpc,
		prodioc.InitTasks,
		prodioc.Crons,
		wire.Struct(new(testioc.App), "*"),
	)

	return new(testioc.App)
}
