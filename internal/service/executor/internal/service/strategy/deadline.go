package strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// DeadlineSendStrategy 截止日期发送策略
type DeadlineSendStrategy struct {
	notificationSvc notificationsvc.Service
}

// newDeadlineStrategy 创建截止日期发送策略
func newDeadlineStrategy(notificationSvc notificationsvc.Service) *DeadlineSendStrategy {
	return &DeadlineSendStrategy{
		notificationSvc: notificationSvc,
	}
}

// Send 截止日期发送通知
func (s *DeadlineSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	now := time.Now()
	notificationSvcDomains := make([]notificationsvc.Notification, len(ns))
	for i := range ns {
		// 根据发送策略，计算调度窗口
		deadlineTime := ns[i].SendStrategyConfig.DeadlineTime
		if deadlineTime.Before(now) {
			return nil, fmt.Errorf("%w: 截止日期已过期", ErrInvalidParameter)
		}

		// 设置时间窗口: 从现在到截止日期
		ns[i].Notification.ScheduledSTime = now.UnixMilli()
		ns[i].Notification.ScheduledETime = deadlineTime.UnixMilli()

		// 获取notification模块的领域模型
		notificationSvcDomains[i] = ns[i].Notification
	}

	// 创建通知记录
	createdNotifications, err := s.notificationSvc.BatchCreate(ctx, notificationSvcDomains)
	if err != nil {
		return nil, fmt.Errorf("创建截止日期通知失败: %w", err)
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
