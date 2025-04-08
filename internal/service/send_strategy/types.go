package send_strategy

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/sender"
)

// 定义统一的错误类型
var (
	// ErrInvalidParameter 表示参数错误
	ErrInvalidParameter = errors.New("参数错误")
)

// SendStrategy 发送策略接口
type SendStrategy interface {
	// Send 发送通知
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

// BatchSend 发送通知
// 根据通知中指定的策略类型选择合适的发送策略
func (d *Dispatcher) BatchSend(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	const first = 0
	strategyType := ns[first].SendStrategyConfig.Type

	// 获取策略
	strategy, ok := d.strategies[strategyType]
	if !ok {
		return nil, fmt.Errorf("%w: 无效的发送策略 %s", ErrInvalidParameter, strategyType)
	}

	// 执行发送
	return strategy.BatchSend(ctx, ns)
}
