package send_strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// DeadlineSendStrategy 截止日期发送策略
type DeadlineSendStrategy struct {
	repo repository.NotificationRepository
}

// newDeadlineStrategy 创建截止日期发送策略
func newDeadlineStrategy(repo repository.NotificationRepository) *DeadlineSendStrategy {
	return &DeadlineSendStrategy{
		repo: repo,
	}
}

// Send 单条发送通知
func (s *DeadlineSendStrategy) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	now := time.Now()

	// 根据发送策略，计算调度窗口
	deadlineTime := notification.SendStrategyConfig.DeadlineTime
	if deadlineTime.Before(now) {
		return domain.SendResponse{}, fmt.Errorf("%w: 截止日期已过期", ErrInvalidParameter)
	}

	// 设置时间窗口: 从现在到截止日期
	notification.ScheduledSTime = now.UnixMilli()
	notification.ScheduledETime = deadlineTime.UnixMilli()

	// 创建通知记录
	created, err := s.repo.Create(ctx, notification)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("创建截止日期通知失败: %w", err)
	}

	// 仅创建通知记录，等待定时任务扫描发送
	return domain.SendResponse{
		NotificationID: created.ID,
		Status:         created.Status,
	}, nil
}

// BatchSend 批量发送通知，其中每个通知的发送策略必须相同
func (s *DeadlineSendStrategy) BatchSend(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error) {
	if len(notifications) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	now := time.Now()
	for i := range notifications {
		// 根据发送策略，计算调度窗口
		deadlineTime := notifications[i].SendStrategyConfig.DeadlineTime
		if deadlineTime.Before(now) {
			return nil, fmt.Errorf("%w: 截止日期已过期", ErrInvalidParameter)
		}

		// 设置时间窗口: 从现在到截止日期
		notifications[i].ScheduledSTime = now.UnixMilli()
		notifications[i].ScheduledETime = deadlineTime.UnixMilli()
	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
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
