package send_strategy

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/errs"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// ScheduledSendStrategy 定时发送策略
type ScheduledSendStrategy struct {
	repo repository.NotificationRepository
}

// newScheduledStrategy 创建定时发送策略
func newScheduledStrategy(repo repository.NotificationRepository) *ScheduledSendStrategy {
	return &ScheduledSendStrategy{
		repo: repo,
	}
}

// Send 单条发送通知
func (s *ScheduledSendStrategy) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// 根据发送策略，计算调度窗口
	scheduledTime := notification.SendStrategyConfig.ScheduledTime
	if scheduledTime.Before(time.Now()) {
		return domain.SendResponse{}, fmt.Errorf("%w: 定时参数已过期", errs.ErrInvalidParameter)
	}
	notification.ScheduledSTime = scheduledTime.UnixMilli()
	notification.ScheduledETime = scheduledTime.UnixMilli()

	// 创建通知记录
	created, err := s.repo.Create(ctx, notification)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("创建延迟通知失败: %w", err)
	}

	return domain.SendResponse{
		NotificationID: created.ID,
		Status:         created.Status,
	}, nil
}

// BatchSend 批量发送通知，其中每个通知的发送策略必须相同
func (s *ScheduledSendStrategy) BatchSend(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	for i := range ns {
		// 根据发送策略，计算调度窗口
		scheduledTime := ns[i].SendStrategyConfig.ScheduledTime
		if scheduledTime.Before(time.Now()) {
			return nil, fmt.Errorf("%w: 定时参数已过期", errs.ErrInvalidParameter)
		}
		ns[i].ScheduledSTime = scheduledTime.UnixMilli()
		ns[i].ScheduledETime = scheduledTime.UnixMilli()
	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, ns)
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
