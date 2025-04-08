package strategy

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"time"
)

// ImmediateSendStrategy 立即发送策略
type ImmediateSendStrategy struct {
	repo   repository.NotificationRepository
	sender sender.NotificationSender
}

// newImmediateStrategy 创建立即发送策略
func newImmediateStrategy(repo repository.NotificationRepository, sender sender.NotificationSender) *ImmediateSendStrategy {
	return &ImmediateSendStrategy{
		repo:   repo,
		sender: sender,
	}
}

// Send 立即发送通知
func (s *ImmediateSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	now := time.Now()
	for i := range ns {
		// 根据发送策略，计算调度窗口
		ns[i].ScheduledSTime = now.UnixMilli()
		ns[i].ScheduledETime = now.Add(time.Hour).UnixMilli() // 兜底
	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, ns)
	if err == nil {
		// 立即发送
		return s.sender.Send(ctx, createdNotifications)
	}

	// 批量操作直接返回错误
	if len(ns) > 1 || !errors.Is(err, repository.ErrNotificationDuplicate) {
		return nil, fmt.Errorf("创建通知失败: %w", err)
	}

	// 单个操作， 错误为索引冲突，表示业务方重试
	const first = 0
	n := ns[first]
	notifications, err := s.repo.GetByKeys(ctx, n.BizID, n.Key)
	if err != nil {
		return nil, fmt.Errorf("获取通知失败: %w", err)
	}

	// 已经发送成功了
	if notifications[first].Status == domain.StatusSucceeded {
		return []domain.SendResponse{
			{
				NotificationID: notifications[first].ID,
				Status:         notifications[first].Status,
			},
		}, nil
	}

	// 事务消息直接返回错误
	if notifications[first].Status == domain.StatusPrepare ||
		notifications[first].Status == domain.StatusCanceled {
		return nil, fmt.Errorf("事务消息")
	}

	// 更新通知状态为PENDING同时获取乐观锁（版本号）
	notifications[first].Status = domain.StatusPending
	err = s.repo.UpdateStatus(ctx, notifications[first].ID, notifications[first].Status, notifications[first].Version)
	if err != nil {
		return nil, fmt.Errorf("更新通知状态失败: %w", err)
	}

	// 再次立即发送
	return s.sender.Send(ctx, createdNotifications)
}
