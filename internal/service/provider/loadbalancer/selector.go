package loadbalancer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
)

type Selector struct {
	providers []*mprovider  // 被封装的provider列表
	count     int64         // 轮询计数器
	mu        *sync.RWMutex // 保护providers的并发访问
}

func NewSelector(providers []provider.Provider, bufferLen int) *Selector {
	if bufferLen <= 0 {
		bufferLen = 10 // 默认缓冲区长度
	}

	// 预分配足够的容量避免扩容
	mproviders := make([]*mprovider, 0, len(providers))
	for _, p := range providers {
		mp := newMprovider(p, bufferLen)
		mproviders = append(mproviders, &mp)
	}

	return &Selector{
		providers: mproviders,
		count:     0,
		mu:        &sync.RWMutex{},
	}
}

func (s *Selector) Next(_ context.Context, _ domain.Notification) (provider.Provider, error) {
	// 获取provider列表的快照，由于长度不会改变，只需一次加锁操作
	providers := s.providers
	providerCount := len(providers)

	if providerCount == 0 {
		return nil, ErrNoProvidersAvailable
	}

	// 原子操作获取并递增计数，确保均匀分配负载
	current := atomic.AddInt64(&s.count, 1) - 1

	// 轮询所有provider
	for i := 0; i < providerCount; i++ {
		// 计算当前要使用的provider索引
		idx := (int(current) + i) % providerCount

		// 由于providers长度不变，可以安全地直接访问
		pro := providers[idx]

		// 检查provider是否健康
		if pro != nil && pro.healthy.Load() {
			return pro, nil
		}
	}

	// 所有provider都不健康或发送失败
	return nil, ErrNoHealthyProvider
}

// MonitorProvidersHealth 监控不健康的provider，当它们恢复健康时更新状态
// 参数:
//   - ctx: 上下文，用于取消监控
//   - checkInterval: 检查间隔时间，决定多久检查一次不健康的provider
func (s *Selector) MonitorProvidersHealth(ctx context.Context, checkInterval time.Duration) {
	if checkInterval <= 0 {
		checkInterval = defaultHealthyInterval // 默认检查间隔
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndUpdateProvidersHealth()
		}
	}
}

// checkAndUpdateProvidersHealth 检查所有不健康的provider并更新它们的状态
func (s *Selector) checkAndUpdateProvidersHealth() {
	for _, pro := range s.providers {
		// 只检查不健康的provider
		if pro != nil && !pro.healthy.Load() {
			pro.checkAndRecover()
		}
	}
}
