package send_strategy

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/errs"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/sender"
)

// SendStrategy 发送策略接口
type SendStrategy interface {
	// Send 单条发送通知
	Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error)
	// BatchSend 批量发送通知，其中每个通知的发送策略必须相同
	BatchSend(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error)
}

// Dispatcher 通知发送分发器
// 根据通知的策略类型选择合适的发送策略
type Dispatcher struct {
	sender     sender.NotificationSender
	strategies map[domain.SendStrategyType]SendStrategy
}

// NewDispatcher 创建通知发送分发器
func NewDispatcher(
	strategies map[domain.SendStrategyType]SendStrategy,
	sender sender.NotificationSender) SendStrategy {
	return &Dispatcher{
		sender:     sender,
		strategies: strategies,
	}
}

// Send 发送通知
func (d *Dispatcher) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// 获取策略
	strategy, ok := d.strategies[notification.SendStrategyConfig.Type]
	if !ok {
		return domain.SendResponse{}, fmt.Errorf("%w: 无效的发送策略 %s", errs.ErrInvalidParameter, notification.SendStrategyConfig.Type)
	}
	// 执行发送
	return strategy.Send(ctx, notification)
}

// BatchSend 批量发送通知
func (d *Dispatcher) BatchSend(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	const first = 0
	strategyType := ns[first].SendStrategyConfig.Type

	// 获取策略
	strategy, ok := d.strategies[strategyType]
	if !ok {
		return nil, fmt.Errorf("%w: 无效的发送策略 %s", errs.ErrInvalidParameter, strategyType)
	}

	// 执行发送
	return strategy.BatchSend(ctx, ns)
}
