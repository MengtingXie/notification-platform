package strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// DelayedSendStrategy 延迟发送策略
type DelayedSendStrategy struct {
	notificationSvc notificationsvc.Service
}

// newDelayedStrategy 创建延迟发送策略
func newDelayedStrategy(notificationSvc notificationsvc.Service) *DelayedSendStrategy {
	return &DelayedSendStrategy{
		notificationSvc: notificationSvc,
	}
}

// Send 延迟发送通知
func (s *DelayedSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	notificationSvcDomains := make([]notificationsvc.Notification, len(ns))
	for i := range ns {
		// 检查延迟参数
		if ns[i].SendStrategyConfig.DelaySeconds <= 0 {
			return nil, fmt.Errorf("%w: 延迟发送策略必须指定正数的延迟秒数", ErrInvalidParameter)
		}

		// 根据发送策略，计算调度窗口
		delayDuration := time.Duration(ns[i].SendStrategyConfig.DelaySeconds) * time.Second
		now := time.Now()
		scheduledTime := now.Add(delayDuration)
		ns[i].Notification.ScheduledSTime = now.UnixMilli()
		ns[i].Notification.ScheduledETime = scheduledTime.UnixMilli()

		// 获取notification模块的领域模型
		notificationSvcDomains[i] = ns[i].Notification
	}

	// 创建通知记录
	createdNotifications, err := s.notificationSvc.BatchCreateNotifications(ctx, notificationSvcDomains)
	if err != nil {
		return nil, fmt.Errorf("创建延迟通知失败: %w", err)
	}

	// 仅创建通知记录，等待定时任务扫描发送
	responses := make([]domain.SendResponse, len(createdNotifications))
	for i := range createdNotifications {
		responses[i] = domain.SendResponse{
			NotificationID: createdNotifications[i].ID,
			Status:         createdNotifications[i].Status,
		}
	}
	return responses, nil
}
