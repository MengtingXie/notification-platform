package loadbalancer

import (
	"context"
	"sync"
	"sync/atomic"

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
