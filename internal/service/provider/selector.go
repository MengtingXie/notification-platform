package provider

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	"gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"sort"
	"sync"

	"github.com/gotomicro/ego/core/elog"
)

var (
	ErrNoAvailableProvider = errors.New("无可用供应商")
	ErrChannelUnsupported  = errors.New("未支持的渠道")
)

// Selector 供应商选择器接口
type Selector interface {
	// Next 获取下一个供应商
	Next(ctx context.Context, notification domain.Notification) (Provider, error)
	// Reset 重置选择器状态
	Reset()
}

// selector 供应商选择器实现
type selector struct {
	mu sync.Mutex // 添加互斥锁保护并发访问

	providerSvc manage.ManageService

	providers  map[domain.Channel]map[string]Provider
	smsClients map[string]sms.Client

	// 当前处理状态
	providerConfigs []domain.Provider
	currentIndex    int

	logger *elog.Component
}

// newSelector 创建供应商选择器
func newSelector(providerSvc manage.ManageService, smsClients map[string]sms.Client) Selector {
	return &selector{
		providerSvc:  providerSvc,
		smsClients:   smsClients,
		currentIndex: 0,
	}
}

// Next 获取下一个供应商
func (s *selector) Next(ctx context.Context, notification domain.Notification) (Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 初始化供应商配置列表
	if err := s.initProviderConfigs(ctx, notification.Channel); err != nil {
		return nil, err
	}

	// 检查是否还有可用供应商
	for {
		if s.currentIndex >= len(s.providerConfigs) {
			return nil, fmt.Errorf("%w", ErrNoAvailableProvider)
		}

		// 获取下一个供应商
		providerInfo := s.providerConfigs[s.currentIndex]
		s.currentIndex++

		channelProvider, ok := s.providers[notification.Channel]
		if !ok {
			s.logger.Warn("根据notification.Channel获取供应商集合失败（未支持的渠道）",
				elog.Any("Channel", notification.Channel),
			)
			return nil, fmt.Errorf("%w: %w: %s", ErrNoAvailableProvider, ErrChannelUnsupported, notification.Channel)
		}

		provider, ok := channelProvider[providerInfo.Name]
		if !ok {
			s.logger.Warn("根据providerInfo.Name获取渠道供应商失败",
				elog.Any("Name", providerInfo.Name),
			)
			continue
		}

		return provider, nil
	}
}

// initProviderConfigs 初始化供应商列表
func (s *selector) initProviderConfigs(ctx context.Context, channelType domain.Channel) error {
	// 注意：该方法应该在持有锁的情况下调用
	providerConfigs, err := s.providerSvc.GetProvidersByChannel(ctx, channelType)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoAvailableProvider, err)
	}

	if len(providerConfigs) == 0 {
		return fmt.Errorf("%w", ErrNoAvailableProvider)
	}

	// 根据权重排序
	sort.Slice(providerConfigs, func(i, j int) bool {
		return providerConfigs[i].Weight > providerConfigs[j].Weight
	})

	// TODO: 根据 providerConfigs[0].QpsLimit, providerConfigs[0].DailyLimit 过滤供应商
	// 达到限制的要动态地从 providerConfigs 中 删除

	// 判断过滤后是否有可用供应商
	if len(providerConfigs) == 0 {
		return fmt.Errorf("%w", ErrNoAvailableProvider)
	}

	s.providerConfigs = providerConfigs
	return s.initProviders(channelType)
}

// initProviders 根据供应商配置和传入的渠道客户端初始化供应商对象
func (s *selector) initProviders(channelType domain.Channel) error {
	s.providers = make(map[domain.Channel]map[string]Provider)
	for i := range s.providerConfigs {
		if channelType == domain.ChannelSMS {
			switch s.providerConfigs[i].Code {
			case "tencentCloud":
				// 渠道类型: 供应商名称(aliyun-sms或aliyun): 供应商对象
				s.providers[channelType][s.providerConfigs[i].Name] = NewSMSProvider(
					s.providerConfigs[i].Name,
					s.templateSvc,
					s.smsClients[s.providerConfigs[i].Code])
			case "aliyun":
				s.providers[channelType][s.providerConfigs[i].Name] = NewSMSProvider(
					s.providerConfigs[i].Name,
					s.templateSvc,
					s.smsClients[s.providerConfigs[i].Code])
			}
		}
	}
	return nil
}

// Reset 重置选择器状态
func (s *selector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.providerConfigs = nil
	s.currentIndex = 0
}
