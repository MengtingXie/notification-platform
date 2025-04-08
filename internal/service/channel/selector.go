package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"sync"
	"time"

	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
)

var (
	ErrNoAvailableChannel = errors.New("没有可用的渠道")
	ErrChannelSendFailed  = errors.New("渠道发送失败")
	ErrBizConfigNotFound  = errors.New("未找到业务配置信息")
)

const (
	MAXRETRIES = 3
)

// RetryPolicy 重试策略配置
type RetryPolicy struct {
	MaxRetries     int   `json:"maxRetries"`     // 最大重试次数
	RetryIntervals []int `json:"retryIntervals"` // 重试间隔时间(毫秒)
}

// Config 渠道配置
type Config struct {
	Channels []struct {
		Channel  string `json:"channel"`  // 渠道类型
		Priority int    `json:"priority"` // 优先级，数字越小优先级越高
		Enabled  bool   `json:"enabled"`  // 是否启用
	} `json:"channels"`
}

// Selector 渠道选择器接口
type Selector interface {
	// Reset 重置选择器状态
	Reset()
	// Next 获取下一个可用的渠道实现
	Next(ctx context.Context, notification domain.Notification) (Channel, error)
}

// selector 渠道选择器实现
type selector struct {
	mu sync.Mutex

	// 配置服务
	configSvc configsvc.BusinessConfigService

	// 供应商
	providerDispatcher provider.Provider

	// 缓存的业务配置，按业务ID索引
	bizConfigCache map[int64]domain.BusinessConfig

	// 缓存的重试策略，按业务ID索引
	retryPolicyCache map[int64]RetryPolicy

	// 缓存的渠道列表，按业务ID和通知渠道索引
	channelListCache map[string][]domain.Channel

	// 当前处理的通知状态
	currentState struct {
		notification       domain.Notification
		attemptedChannels  map[domain.Channel]bool
		currentChannelType domain.Channel
		currentChannelIdx  int
	}
}

// newSelector 创建渠道选择器
func newSelector(providerDispatcher provider.Provider, configSvc configsvc.BusinessConfigService) Selector {
	return &selector{
		configSvc:          configSvc,
		providerDispatcher: providerDispatcher,
		bizConfigCache:     make(map[int64]domain.BusinessConfig),
		retryPolicyCache:   make(map[int64]RetryPolicy),
		channelListCache:   make(map[string][]domain.Channel),
	}
}

// Reset 重置选择器状态
func (s *selector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 重置当前处理的通知状态
	s.currentState.notification = domain.Notification{}
	s.currentState.attemptedChannels = make(map[domain.Channel]bool)
	s.currentState.currentChannelType = ""
	s.currentState.currentChannelIdx = 0
}

// Next 获取下一个可用渠道
func (s *selector) Next(ctx context.Context, notification domain.Notification) (Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果是新的通知或不同的通知，需要初始化状态
	if s.currentState.notification.ID != notification.ID ||
		s.currentState.notification.BizID != notification.BizID {
		s.currentState.notification = notification
		s.currentState.attemptedChannels = make(map[domain.Channel]bool)
		s.currentState.currentChannelIdx = 0
	}

	// 获取可用渠道列表
	channels, err := s.getAvailableChannels(ctx, notification)
	if err != nil {
		return nil, err
	}

	// 遍历寻找下一个未尝试过的渠道
	for s.currentState.currentChannelIdx < len(channels) {

		channelType := channels[s.currentState.currentChannelIdx]
		s.currentState.currentChannelIdx++

		if !s.currentState.attemptedChannels[channelType] {
			// 标记为已尝试
			s.currentState.attemptedChannels[channelType] = true
			s.currentState.currentChannelType = channelType

			// 创建渠道处理器
			return s.createChannelHandler(notification.BizID, channelType), nil
		}
	}

	// 所有渠道都已尝试
	return nil, fmt.Errorf("%w", ErrNoAvailableChannel)
}

// getAvailableChannels 获取可用渠道列表
func (s *selector) getAvailableChannels(ctx context.Context, notification domain.Notification) ([]domain.Channel, error) {
	bizID := notification.BizID
	cacheKey := getCacheKey(bizID, notification.Channel)

	// 检查缓存
	if channels, ok := s.channelListCache[cacheKey]; ok {
		return channels, nil
	}

	// 获取业务配置
	bizConfig, err := s.getBizConfig(ctx, bizID)
	if err != nil {
		return nil, err
	}

	// 解析渠道配置
	channels := s.unmarshalChannelConfig(bizConfig.ChannelConfig, notification.Channel)

	// 使用默认渠道列表
	s.channelListCache[cacheKey] = channels
	return channels, nil
}

// getCacheKey 生成缓存键
func getCacheKey(bizID int64, channel domain.Channel) string {
	return fmt.Sprintf("%d:%s", bizID, channel)
}

// getBizConfig 获取业务配置
func (s *selector) getBizConfig(ctx context.Context, bizID int64) (domain.BusinessConfig, error) {
	// 检查缓存
	if config, ok := s.bizConfigCache[bizID]; ok {
		return config, nil
	}

	// 从服务获取
	config, err := s.configSvc.GetByID(ctx, bizID)
	if err != nil {
		return domain.BusinessConfig{}, fmt.Errorf("%w: %w", ErrBizConfigNotFound, err)
	}

	// 缓存结果
	s.bizConfigCache[bizID] = config
	s.retryPolicyCache[bizID] = s.unmarshalRetryPolicy(config.RetryPolicy)
	return config, nil
}

// unmarshalRetryPolicy 解析重试策略
func (s *selector) unmarshalRetryPolicy(retryPolicyStr string) RetryPolicy {
	defaultRetryPolicy := RetryPolicy{
		MaxRetries:     MAXRETRIES,
		RetryIntervals: []int{1000, 2000, 3000},
	}

	if retryPolicyStr == "" {
		return defaultRetryPolicy
	}

	var retryPolicy RetryPolicy

	err := json.Unmarshal([]byte(retryPolicyStr), &retryPolicy)
	if err != nil {
		return defaultRetryPolicy
	}
	if len(retryPolicy.RetryIntervals) == 0 {
		// 默认间隔1秒
		retryPolicy.RetryIntervals = make([]int, retryPolicy.MaxRetries)
		for i := 0; i < retryPolicy.MaxRetries; i++ {
			retryPolicy.RetryIntervals[i] = 1000
		}
	}
	return defaultRetryPolicy
}

func (s *selector) unmarshalChannelConfig(channelConfigStr string, channel domain.Channel) []domain.Channel {
	channels := make([]domain.Channel, 0)

	// 首先添加通知指定的渠道作为默认渠道
	if channel != "" {
		channels = append(channels, channel)
	}

	// 添加配置中的其他渠道
	var channelConfig Config
	if err := json.Unmarshal([]byte(channelConfigStr), &channelConfig); err != nil {
		return channels
	}
	for _, ch := range channelConfig.Channels {
		if ch.Enabled && domain.Channel(ch.Channel) != channel {
			channels = append(channels, domain.Channel(ch.Channel))
		}
	}
	return channels
}

// createChannelHandler 创建渠道处理器
func (s *selector) createChannelHandler(bizID int64, channelType domain.Channel) Channel {
	// 获取重试策略
	var retryPolicy RetryPolicy
	if policy, ok := s.retryPolicyCache[bizID]; ok {
		retryPolicy = policy
	} else {
		// 默认重试策略
		retryPolicy = RetryPolicy{
			MaxRetries:     MAXRETRIES,
			RetryIntervals: []int{1000, 2000, 3000},
		}
	}
	return &channelHandler{
		ChannelName:        string(channelType),
		providerDispatcher: s.providerDispatcher,
		retryPolicy:        retryPolicy,
	}
}

// channelHandler 特定渠道的处理器
type channelHandler struct {
	ChannelName        string
	providerDispatcher provider.Provider
	retryPolicy        RetryPolicy
}

// Send 使用特定渠道发送通知
func (ch *channelHandler) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// 创建该渠道的通知副本
	channelNotification := notification
	channelNotification.Channel = domain.Channel(ch.ChannelName)

	// 应用重试策略，对同一个渠道进行多次尝试
	var lastError error
	var retryCount int8

	for retry := 0; retry < ch.retryPolicy.MaxRetries; retry++ {
		// 调用provider层处理具体发送
		resp, err := ch.providerDispatcher.Send(ctx, channelNotification)

		if err == nil {
			// 发送成功
			resp.RetryCount = retryCount
			return resp, nil
		}

		// 发送失败，记录错误
		lastError = err
		retryCount++

		// 最后一次失败，不需要等待
		if retry >= ch.retryPolicy.MaxRetries-1 {
			break
		}

		// 获取重试间隔
		var interval int
		if retry < len(ch.retryPolicy.RetryIntervals) {
			interval = ch.retryPolicy.RetryIntervals[retry]
		} else {
			// 默认1秒
			interval = 1000
		}

		// 等待重试间隔
		select {
		case <-ctx.Done():
			// 上下文取消
			return domain.SendResponse{RetryCount: retryCount}, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Millisecond):
			// 继续重试
		}
	}
	// 所有重试都失败
	return domain.SendResponse{RetryCount: retryCount}, fmt.Errorf("%w: %w", ErrChannelSendFailed, lastError)
}
