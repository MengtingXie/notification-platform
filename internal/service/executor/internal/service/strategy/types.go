package strategy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/sender"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// 定义统一的错误类型
var (
	// ErrInvalidParameter 表示参数错误
	ErrInvalidParameter = errors.New("参数错误")
)

// SendStrategy 发送策略接口
type SendStrategy interface {
	// Send 发送通知
	Send(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error)
}

// Dispatcher 通知发送分发器
// 根据通知的策略类型选择合适的发送策略
type Dispatcher struct {
	notificationSvc notificationsvc.Service
	sender          sender.NotificationSender

	// 使用sync.Once保证线程安全的单例初始化
	immediateStrategyOnce  sync.Once
	scheduledStrategyOnce  sync.Once
	delayedStrategyOnce    sync.Once
	timeWindowStrategyOnce sync.Once

	// 缓存已创建的策略实例
	immediateStrategy  *ImmediateSendStrategy
	scheduledStrategy  *ScheduledSendStrategy
	delayedStrategy    *DelayedSendStrategy
	timeWindowStrategy *TimeWindowSendStrategy
}

// NewDispatcher 创建通知发送分发器
func NewDispatcher(notificationSvc notificationsvc.Service, sender sender.NotificationSender) SendStrategy {
	return &Dispatcher{
		notificationSvc: notificationSvc,
		sender:          sender,
	}
}

// Send 发送通知
// 根据通知中指定的策略类型选择合适的发送策略
func (d *Dispatcher) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	const first = 0
	// 获取对应策略
	strategy, err := d.getStrategy(ns[first].SendStrategyConfig.Type)
	if err != nil {
		return nil, err
	}

	// 执行发送
	return strategy.Send(ctx, ns)
}

// getStrategy 获取发送策略
func (d *Dispatcher) getStrategy(strategyType domain.SendStrategyType) (SendStrategy, error) {
	switch strategyType {
	case domain.SendStrategyImmediate:
		return d.getImmediateStrategy(), nil
	case domain.SendStrategyScheduled:
		return d.getScheduledStrategy(), nil
	case domain.SendStrategyDelayed:
		return d.getDelayedStrategy(), nil
	case domain.SendStrategyTimeWindow:
		return d.getTimeWindowStrategy(), nil
	default:
		return nil, fmt.Errorf("%w: 无效的发送策略类型 %s", ErrInvalidParameter, strategyType)
	}
}

// getImmediateStrategy 获取立即发送策略（线程安全的单例模式）
func (d *Dispatcher) getImmediateStrategy() *ImmediateSendStrategy {
	d.immediateStrategyOnce.Do(func() {
		d.immediateStrategy = newImmediateStrategy(d.notificationSvc, d.sender)
	})
	return d.immediateStrategy
}

// getScheduledStrategy 获取定时发送策略（线程安全的单例模式）
func (d *Dispatcher) getScheduledStrategy() *ScheduledSendStrategy {
	d.scheduledStrategyOnce.Do(func() {
		d.scheduledStrategy = newScheduledStrategy(d.notificationSvc)
	})
	return d.scheduledStrategy
}

// getDelayedStrategy 获取延迟发送策略（线程安全的单例模式）
func (d *Dispatcher) getDelayedStrategy() *DelayedSendStrategy {
	d.delayedStrategyOnce.Do(func() {
		d.delayedStrategy = newDelayedStrategy(d.notificationSvc)
	})
	return d.delayedStrategy
}

// getTimeWindowStrategy 获取时间窗口发送策略（线程安全的单例模式）
func (d *Dispatcher) getTimeWindowStrategy() *TimeWindowSendStrategy {
	d.timeWindowStrategyOnce.Do(func() {
		d.timeWindowStrategy = newTimeWindowStrategy(d.notificationSvc)
	})
	return d.timeWindowStrategy
}
