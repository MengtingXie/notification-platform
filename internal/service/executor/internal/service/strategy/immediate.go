package strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/sender"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// ImmediateSendStrategy 立即发送策略
type ImmediateSendStrategy struct {
	notificationSvc notificationsvc.Service
	sender          sender.NotificationSender
}

// newImmediateStrategy 创建立即发送策略
func newImmediateStrategy(notificationSvc notificationsvc.Service, sender sender.NotificationSender) *ImmediateSendStrategy {
	return &ImmediateSendStrategy{
		notificationSvc: notificationSvc,
		sender:          sender,
	}
}

// Send 立即发送通知
func (s *ImmediateSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	notificationSvcDomains := make([]notificationsvc.Notification, len(ns))
	for i := range ns {
		// 根据发送策略，计算调度窗口
		now := time.Now()
		ns[i].Notification.ScheduledSTime = now.UnixMilli()
		ns[i].Notification.ScheduledETime = now.Add(time.Hour).UnixMilli() // 兜底

		// 获取notification模块的领域模型
		notificationSvcDomains[i] = ns[i].Notification
	}

	// 创建通知记录
	createdNotifications, err := s.notificationSvc.BatchCreateNotifications(ctx, notificationSvcDomains)
	if err != nil {
		return nil, fmt.Errorf("创建通知失败: %w", err)
	}

	// 立即发送
	return s.sender.Send(ctx, createdNotifications)
}
