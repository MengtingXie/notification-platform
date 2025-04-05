package strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// TimeWindowSendStrategy 时间窗口发送策略
type TimeWindowSendStrategy struct {
	notificationSvc notificationsvc.Service
}

// newTimeWindowStrategy 创建时间窗口发送策略
func newTimeWindowStrategy(notificationSvc notificationsvc.Service) *TimeWindowSendStrategy {
	return &TimeWindowSendStrategy{
		notificationSvc: notificationSvc,
	}
}

// Send 在指定时间窗口内发送通知
func (s *TimeWindowSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	// 校验时间窗口
	now := time.Now().UnixMilli()
	notificationSvcDomains := make([]notificationsvc.Notification, len(ns))
	for i := range ns {
		// 根据发送策略，计算调度窗口
		startTime := ns[i].SendStrategyConfig.StartTimeMilliseconds
		endTime := ns[i].SendStrategyConfig.EndTimeMilliseconds

		if startTime <= 0 || startTime >= endTime {
			return nil, fmt.Errorf("%w: 时间窗口开始时间应该大于0且小于结束时间", ErrInvalidParameter)
		}

		if endTime <= now {
			return nil, fmt.Errorf("%w: 时间窗口结束时间应该大于当前时间", ErrInvalidParameter)
		}

		ns[i].Notification.ScheduledSTime = startTime
		ns[i].Notification.ScheduledETime = endTime

		// 获取notification模块的领域模型
		notificationSvcDomains[i] = ns[i].Notification
	}

	// 创建通知记录
	createdNotifications, err := s.notificationSvc.BatchCreate(ctx, notificationSvcDomains)
	if err != nil {
		return nil, fmt.Errorf("创建时间窗口通知失败: %w", err)
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
