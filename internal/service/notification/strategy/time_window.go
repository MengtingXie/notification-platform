package strategy

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"time"
)

// TimeWindowSendStrategy 时间窗口发送策略
type TimeWindowSendStrategy struct {
	repo repository.NotificationRepository
}

// newTimeWindowStrategy 创建时间窗口发送策略
func newTimeWindowStrategy(repo repository.NotificationRepository) *TimeWindowSendStrategy {
	return &TimeWindowSendStrategy{
		repo: repo,
	}
}

// Send 在指定时间窗口内发送通知
func (s *TimeWindowSendStrategy) Send(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {
	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	// 校验时间窗口
	now := time.Now().UnixMilli()
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

		ns[i].ScheduledSTime = startTime
		ns[i].ScheduledETime = endTime

	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, ns)
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
