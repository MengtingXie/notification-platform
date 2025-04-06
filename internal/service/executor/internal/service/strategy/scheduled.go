package strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// ScheduledSendStrategy 定时发送策略
type ScheduledSendStrategy struct {
	notificationSvc notificationsvc.Service
}

// newScheduledStrategy 创建定时发送策略
func newScheduledStrategy(notificationSvc notificationsvc.Service) *ScheduledSendStrategy {
	return &ScheduledSendStrategy{
		notificationSvc: notificationSvc,
	}
}

// Send 定时发送通知
func (s *ScheduledSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	notificationSvcDomains := make([]notificationsvc.Notification, len(ns))
	for i := range ns {
		// 根据发送策略，计算调度窗口
		scheduledTime := ns[i].SendStrategyConfig.ScheduledTime
		if scheduledTime.Before(time.Now()) {
			return nil, fmt.Errorf("%w: 定时参数已过期", ErrInvalidParameter)
		}
		ns[i].Notification.ScheduledSTime = scheduledTime.UnixMilli()
		ns[i].Notification.ScheduledETime = scheduledTime.UnixMilli()

		// 获取notification模块的领域模型
		notificationSvcDomains[i] = ns[i].Notification
	}

	// 创建通知记录
	createdNotifications, err := s.notificationSvc.BatchCreate(ctx, notificationSvcDomains)
	if err != nil {
		return nil, fmt.Errorf("创建定时通知失败: %w", err)
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
