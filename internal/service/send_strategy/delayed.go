package send_strategy

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// DelayedSendStrategy 延迟发送策略
type DelayedSendStrategy struct {
	repo repository.NotificationRepository
}

// newDelayedStrategy 创建延迟发送策略
func newDelayedStrategy(repo repository.NotificationRepository) *DelayedSendStrategy {
	return &DelayedSendStrategy{
		repo: repo,
	}
}

// Send 单条发送通知
func (s *DelayedSendStrategy) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {

	// 检查延迟参数
	if notification.SendStrategyConfig.DelaySeconds <= 0 {
		return domain.SendResponse{}, fmt.Errorf("%w: 延迟发送策略必须指定正数的延迟秒数", ErrInvalidParameter)
	}

	// 根据发送策略，计算调度窗口
	delayDuration := time.Duration(notification.SendStrategyConfig.DelaySeconds) * time.Second
	now := time.Now()
	scheduledTime := now.Add(delayDuration)
	notification.ScheduledSTime = now.UnixMilli()
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
func (s *DelayedSendStrategy) BatchSend(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error) {
	if len(notifications) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	for i := range notifications {
		// 检查延迟参数
		if notifications[i].SendStrategyConfig.DelaySeconds <= 0 {
			return nil, fmt.Errorf("%w: 延迟发送策略必须指定正数的延迟秒数", ErrInvalidParameter)
		}

		// 根据发送策略，计算调度窗口
		delayDuration := time.Duration(notifications[i].SendStrategyConfig.DelaySeconds) * time.Second
		now := time.Now()
		scheduledTime := now.Add(delayDuration)
		notifications[i].ScheduledSTime = now.UnixMilli()
		notifications[i].ScheduledETime = scheduledTime.UnixMilli()

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
