package loadbalancer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
)

const (
	defaultTimeout         = time.Second * 5
	defaultHealthyInterval = 30 * time.Second
)

// 定义包级别错误，提高错误处理的一致性
var (
	ErrNoProvidersAvailable = errors.New("no providers available")
	ErrNoHealthyProvider    = errors.New("no healthy provider available")
)

// Provider 实现了基于轮询的负载均衡通知发送
// 它会自动跳过不健康的provider，确保通知可靠发送
type Provider struct {
	providers []*mprovider  // 被封装的provider列表
	count     int64         // 轮询计数器
	mu        *sync.RWMutex // 保护providers的并发访问
}

// NewProvider 创建一个新的负载均衡Provider
// 参数:
//   - providers: 基础provider列表
//   - bufferLen: 健康状态监控的环形缓冲区长度，用于异常检测
func NewProvider(providers []provider.HealthAwareProvider, bufferLen int) *Provider {
	if bufferLen <= 0 {
		bufferLen = 10 // 默认缓冲区长度
	}

	// 预分配足够的容量避免扩容
	mproviders := make([]*mprovider, 0, len(providers))
	for _, p := range providers {
		mp := newMprovider(p, bufferLen)
		mproviders = append(mproviders, &mp)
	}

	return &Provider{
		providers: mproviders,
		count:     0,
		mu:        &sync.RWMutex{},
	}
}

// Send 轮询查找健康的provider来发送通知
// 如果所有provider都不健康，则返回错误
// 前提：p.providers的长度在使用过程中不会改变
func (p *Provider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// 获取provider列表的快照，由于长度不会改变，只需一次加锁操作
	p.mu.RLock()
	providers := p.providers
	providerCount := len(providers)
	p.mu.RUnlock()

	if providerCount == 0 {
		return domain.SendResponse{}, ErrNoProvidersAvailable
	}

	// 原子操作获取并递增计数，确保均匀分配负载
	current := atomic.AddInt64(&p.count, 1) - 1

	// 轮询所有provider
	for i := 0; i < providerCount; i++ {
		// 计算当前要使用的provider索引
		idx := (int(current) + i) % providerCount

		// 由于providers长度不变，可以安全地直接访问
		pro := providers[idx]

		// 检查provider是否健康
		if pro != nil && pro.healthy.Load() {
			// 使用健康的provider发送通知
			resp, err := pro.Send(ctx, notification)
			if err == nil {
				return resp, nil
			}
			// 发送失败继续尝试下一个
		}
	}

	// 所有provider都不健康或发送失败
	return domain.SendResponse{}, ErrNoHealthyProvider
}

// MonitorProvidersHealth 监控不健康的provider，当它们恢复健康时更新状态
// 参数:
//   - ctx: 上下文，用于取消监控
//   - checkInterval: 检查间隔时间，决定多久检查一次不健康的provider
func (p *Provider) MonitorProvidersHealth(ctx context.Context, checkInterval time.Duration) {
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
			p.checkAndUpdateProvidersHealth(ctx)
		}
	}
}

// checkAndUpdateProvidersHealth 检查所有不健康的provider并更新它们的状态
func (p *Provider) checkAndUpdateProvidersHealth(ctx context.Context) {
	p.mu.RLock()
	providers := p.providers
	p.mu.RUnlock()

	for _, provider := range providers {
		// 只检查不健康的provider
		if provider != nil && !provider.healthy.Load() {
			// 创建一个短期上下文，避免健康检查长时间阻塞
			checkCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
			err := provider.CheckHealth(checkCtx)
			cancel()

			if err == nil {
				// 如果健康检查成功，将provider标记为健康
				provider.markHealthy()
			}
		}
	}
}
