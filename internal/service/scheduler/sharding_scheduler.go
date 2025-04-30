package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/batchsize"
	"gitee.com/flycash/notification-platform/internal/pkg/bitring"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"gitee.com/flycash/notification-platform/internal/sharding"
	"github.com/meoying/dlock-go"
)

const (
	ntabName loopjob.CtxKey = "nTab"
)

// ShardingScheduler 通知调度服务实现
type ShardingScheduler struct {
	repo             repository.NotificationRepository
	sender           sender.NotificationSender
	dclient          dlock.Client
	shardingStrategy sharding.ShardingStrategy
	minLoopDuration  time.Duration

	mu              sync.Mutex
	maxLockedTables uint64
	curLockedTables uint64

	batchSize         atomic.Uint64
	batchSizeAdjuster batchsize.Adjuster

	errorEvents *bitring.BitRing
}

// NewShardingScheduler 创建通知调度服务
func NewShardingScheduler(
	repo repository.NotificationRepository,
	notificationSender sender.NotificationSender,
	dclient dlock.Client,
	shardingStrategy sharding.ShardingStrategy,
	minLoopDuration time.Duration,
	maxLockedTables int,
	batchSize int,
	batchSizeAdjuster batchsize.Adjuster,
	errorEvents *bitring.BitRing,
) *ShardingScheduler {
	s := &ShardingScheduler{
		repo:              repo,
		sender:            notificationSender,
		dclient:           dclient,
		shardingStrategy:  shardingStrategy,
		minLoopDuration:   minLoopDuration,
		maxLockedTables:   uint64(maxLockedTables),
		batchSizeAdjuster: batchSizeAdjuster,
		errorEvents:       errorEvents,
	}
	s.batchSize.Store(uint64(batchSize))
	return s
}

// Start 启动调度服务
// 当 ctx 被取消的或者关闭的时候，就会结束循环
func (s *ShardingScheduler) Start(ctx context.Context) {
	const key = "notification_platform_async_sharding_scheduler"
	dbs := s.shardingStrategy.DBs()
	for i := range dbs {
		go loopjob.NewShardingLoopJob(s.dclient, key, s.loop, dbs[i], s.shardingStrategy.TableSuffix()).Run(ctx)
	}
}

func (s *ShardingScheduler) loop(ctx context.Context) error {
	// 检查锁表计数
	s.mu.Lock()
	if s.curLockedTables == s.maxLockedTables {
		s.mu.Unlock()
		return errors.New("已锁定足够数量的表")
	}
	s.curLockedTables++
	s.mu.Unlock()

	// 退出前更新锁表计数
	defer func() {
		s.mu.Lock()
		s.curLockedTables--
		s.mu.Unlock()
	}()

	for {
		// 记录开始执行时间
		start := time.Now()

		// 批量发送已就绪的通知
		err := s.batchSendReadyNotifications(ctx)

		// 记录响应时间
		responseTime := time.Since(start)

		// 记录错误事件
		s.errorEvents.Add(err != nil)
		// 判断错误事件是否已达到预设的条件 —— 连续出现三次错误，或者错误率达到阈值
		if s.errorEvents.IsConditionMet() {
			return errors.New("错误事件出现次数或错误率达到阈值")
		}

		// 根据响应时间调整batchSize
		newBatchSize, err1 := s.batchSizeAdjuster.Adjust(ctx, responseTime)
		if err1 == nil {
			s.batchSize.Store(uint64(newBatchSize))
		}

		// 没有数据时，响应时间非常快，需要等待一段时间
		if responseTime < s.minLoopDuration {
			time.Sleep(s.minLoopDuration - responseTime)
			continue
		}
	}
}

// batchSendReadyNotifications 批量发送已就绪的通知
func (s *ShardingScheduler) batchSendReadyNotifications(ctx context.Context) error {
	const defaultTimeout = 3 * time.Second

	loopCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	loopCtx = s.ctxWithTableName(loopCtx)
	const offset = 0
	notifications, err := s.repo.FindReadyNotifications(loopCtx, offset, int(s.batchSize.Load()))
	if err != nil {
		return err
	}

	if len(notifications) == 0 {
		return nil
	}

	_, err = s.sender.BatchSend(ctx, notifications)
	return err
}

func (s *ShardingScheduler) ctxWithTableName(ctx context.Context) context.Context {
	suffix, _ := loopjob.TabSuffixFromCtx(ctx)
	nTable := fmt.Sprintf("%s_%s", s.shardingStrategy.TablePrefix(), suffix)
	ctx = context.WithValue(ctx, ntabName, nTable)
	return ctx
}

func (s *ShardingScheduler) UpdateMaxLockedTables(maxLockedTables uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxLockedTables = maxLockedTables
}
