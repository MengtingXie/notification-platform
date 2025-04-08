package send_strategy

import (
	"context"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// DefaultSendStrategy 延迟发送策略
type DefaultSendStrategy struct {
	repo repository.NotificationRepository
}

// NewDefaultStrategy 创建延迟发送策略
func NewDefaultStrategy(repo repository.NotificationRepository) *DefaultSendStrategy {
	return &DefaultSendStrategy{
		repo: repo,
	}
}

// Send 单条发送通知
func (s *DefaultSendStrategy) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	notification.SetSendTime()
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
func (s *DefaultSendStrategy) BatchSend(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error) {
	if len(notifications) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	for i := range notifications {
		notifications[i].SetSendTime()
	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
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
