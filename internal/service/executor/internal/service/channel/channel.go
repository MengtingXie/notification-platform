package channel

import (
	"context"

	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/provider"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// Channel 渠道接口
type Channel interface {
	// Send 发送通知
	Send(ctx context.Context, notification notificationsvc.Notification) (domain.SendResponse, error)
}

// Dispatcher 渠道分发器，对外伪装成Channel，作为统一入口
type Dispatcher struct {
	selector Selector
}

// NewDispatcher 创建渠道分发器
func NewDispatcher(
	provider provider.Provider,
	configSvc configsvc.Service,
) Channel {
	return &Dispatcher{
		selector: newSelector(provider, configSvc),
	}
}

func (d *Dispatcher) Send(ctx context.Context, notification notificationsvc.Notification) (domain.SendResponse, error) {
	d.selector.Reset()

	var retryCount int8
	for {
		channel, err := d.selector.Next(ctx, notification)
		if err != nil {
			// 没有可用的渠道
			return domain.SendResponse{RetryCount: retryCount}, err
		}

		// 使用该渠道发送
		resp, err := channel.Send(ctx, notification)
		if err == nil {
			// 发送成功，填写重试次数
			resp.RetryCount += retryCount
			return resp, nil
		}

		// 累加重试次数
		retryCount++
	}
}
