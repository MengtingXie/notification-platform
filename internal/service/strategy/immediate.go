package strategy

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"time"

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
	createdNotifications, err := s.notificationSvc.BatchCreate(ctx, notificationSvcDomains)
	if err == nil {
		// 立即发送
		return s.sender.Send(ctx, createdNotifications)
	}

	// 批量操作直接返回错误
	if len(ns) > 1 || !errors.Is(err, notificationsvc.ErrNotificationDuplicate) {
		return nil, fmt.Errorf("创建通知失败: %w", err)
	}

	// 单个操作， 错误为索引冲突，表示业务方重试
	const first = 0
	n := ns[first].Notification
	notifications, err := s.notificationSvc.GetByKeys(ctx, n.BizID, n.Key)
	if err != nil {
		return nil, fmt.Errorf("获取通知失败: %w", err)
	}

	// 已经发送成功了
	if notifications[first].Status == notificationsvc.SendStatusSucceeded {
		return []domain.SendResponse{
			{
				NotificationID: notifications[first].ID,
				Status:         notifications[first].Status,
			},
		}, nil
	}

	// 事务消息直接返回错误
	if notifications[first].Status == notificationsvc.SendStatusPrepare ||
		notifications[first].Status == notificationsvc.SendStatusCanceled {
		return nil, fmt.Errorf("事务消息")
	}

	// 更新通知状态为PENDING同时获取乐观锁（版本号）
	notifications[first].Status = notificationsvc.SendStatusPending
	err = s.notificationSvc.UpdateStatus(ctx, notifications[first].ID, notifications[first].Status, notifications[first].Version)
	if err != nil {
		return nil, fmt.Errorf("更新通知状态失败: %w", err)
	}

	// 再次立即发送
	return s.sender.Send(ctx, createdNotifications)
}
