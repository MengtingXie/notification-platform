package ioc

import (
	"context"
	"strconv"
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/batchsize"
	"gitee.com/flycash/notification-platform/internal/pkg/bitring"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/scheduler"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"gitee.com/flycash/notification-platform/internal/sharding"
	"github.com/ego-component/eetcd"
	"github.com/gotomicro/ego/core/econf"
	"github.com/meoying/dlock-go"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func InitShardingScheduler(
	repo repository.NotificationRepository,
	notificationSender sender.NotificationSender,
	dclient dlock.Client,
	shardingStrategy sharding.ShardingStrategy,
	etcdClient *eetcd.Component,
) scheduler.NotificationScheduler {
	type BatchSizeAdjusterConfig struct {
		InitBatchSize  int           `yaml:"initBatchSize"`
		MinBatchSize   int           `yaml:"minBatchSize"`
		MaxBatchSize   int           `yaml:"maxBatchSize"`
		AdjustStep     int           `yaml:"adjustStep"`
		CooldownPeriod time.Duration `yaml:"cooldownPeriod"`
		BufferSize     int           `yaml:"bufferSize"`
	}

	type ErrorEventConfig struct {
		BitRingSize      int     `yaml:"bitRingSize"`
		RateThreshold    float64 `yaml:"rateThreshold"`
		ConsecutiveCount int     `yaml:"consecutiveCount"`
	}

	type ShardingSchedulerConfig struct {
		MaxLockedTablesKey string                  `yaml:"maxLockedTablesKey"`
		MaxLockedTables    int                     `yaml:"maxLockedTables"`
		MinLoopDuration    time.Duration           `yaml:"minLoopDuration"`
		BatchSize          int                     `yaml:"batchSize"`
		BatchSizeAdjuster  BatchSizeAdjusterConfig `yaml:"batchSizeAdjuster"`
		ErrorEvents        ErrorEventConfig        `yaml:"errorEvents"`
	}

	var cfg ShardingSchedulerConfig
	if err := econf.UnmarshalKey("sharding_scheduler", &cfg); err != nil {
		panic(err)
	}

	s := scheduler.NewShardingScheduler(
		repo,
		notificationSender,
		dclient,
		shardingStrategy,
		cfg.MinLoopDuration,
		cfg.MaxLockedTables,
		cfg.BatchSize,
		batchsize.NewRingBufferAdjuster(
			cfg.BatchSizeAdjuster.InitBatchSize,
			cfg.BatchSizeAdjuster.MinBatchSize,
			cfg.BatchSizeAdjuster.MaxBatchSize,
			cfg.BatchSizeAdjuster.AdjustStep,
			cfg.BatchSizeAdjuster.CooldownPeriod,
			cfg.BatchSizeAdjuster.BufferSize),
		bitring.NewBitRing(
			cfg.ErrorEvents.BitRingSize,
			cfg.ErrorEvents.RateThreshold,
			cfg.ErrorEvents.ConsecutiveCount,
		),
	)

	// 处理最大锁定表数变更事件
	go func() {
		watchChan := etcdClient.Watch(context.Background(), cfg.MaxLockedTablesKey)
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				if event.Type == clientv3.EventTypePut {
					maxLockedTables, _ := strconv.ParseUint(string(event.Kv.Value), 10, 64)
					s.UpdateMaxLockedTables(maxLockedTables)
				}
			}
		}
	}()

	return s
}
