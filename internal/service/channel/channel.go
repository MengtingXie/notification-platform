package channel

import (
	"context"
	"gitee.com/flycash/notification-platform/internal/domain"
)

// Channel 渠道接口
type Channel interface {
	// Send 发送通知
	Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error)
}

// Dispatcher 渠道分发器，对外伪装成Channel，作为统一入口
type Dispatcher struct {
	channels map[domain.Channel]Channel
}

// NewDispatcher 创建渠道分发器
//func NewDispatcher(
//	provider provider.Provider,
//	configSvc configsvc.BusinessConfigService,
//) Channel {
//	return &Dispatcher{
//		selector: newSelector(provider, configSvc),
//	}
//}

func (d *Dispatcher) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	channel := d.channels[notification.Channel]
	resp, err := channel.Send(ctx, notification)
	// 配额计算一下就可以了

	//if err == nil {
	//	return resp, err
	//}
	//
	//// 切换 backup
	//var bizConfig
	return resp, err
}
