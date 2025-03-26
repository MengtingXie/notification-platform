package service

import (
	"context"
	"errors"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/repository"
)

var (
	ErrInvalidParameter     = errors.New("参数非法")
	ErrNotificationNotFound = errors.New("通知记录不存在")
	ErrChannelDisabled      = errors.New("通知渠道已禁用")
)

// NotificationService 通知服务接口
//
//go:generate mockgen -source=./notification.go -destination=../mocks/notification.mock.go -package=notificationmocks -typed NotificationService
type NotificationService interface {
	// CreateNotification 创建通知记录
	CreateNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error)

	// SendNotification 发送通知
	SendNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error)

	// BatchSendNotifications 批量发送通知
	BatchSendNotifications(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)

	// GetNotificationByID 根据ID获取通知记录
	GetNotificationByID(ctx context.Context, id uint64) (domain.Notification, error)

	// GetNotificationsByBizID 根据业务ID获取通知记录列表
	GetNotificationsByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// UpdateNotificationStatus 更新通知状态
	UpdateNotificationStatus(ctx context.Context, id uint64, bizID int64, status domain.Status) error

	// BatchUpdateNotificationStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateNotificationStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error
}

// notificationService 通知服务实现
type notificationService struct {
	repo repository.NotificationRepository
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{
		repo: repo,
	}
}

// CreateNotification 创建通知
func (s *notificationService) CreateNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	// 参数校验
	if err := s.validateNotification(notification); err != nil {
		return domain.Notification{}, err
	}

	err := s.repo.Create(ctx, notification)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("创建通知记录失败: %w", err)
	}

	return notification, nil
}

// validateNotification 验证通知参数
func (s *notificationService) validateNotification(n domain.Notification) error {
	if n.BizID <= 0 {
		return fmt.Errorf("%w: BizID = %d", ErrInvalidParameter, n.BizID)
	}
	if n.Key == "" {
		return fmt.Errorf("%w: Key = %q", ErrInvalidParameter, n.Key)
	}
	if n.Receiver == "" {
		return fmt.Errorf("%w: Receiver = %q", ErrInvalidParameter, n.Receiver)
	}
	if n.Channel == "" {
		return fmt.Errorf("%w: Channel = %q", ErrInvalidParameter, n.Channel)
	}
	if n.TemplateID <= 0 {
		return fmt.Errorf("%w: TemplateID = %d", ErrInvalidParameter, n.TemplateID)
	}
	if n.TemplateVersionID <= 0 {
		return fmt.Errorf("%w: TemplateVersionID = %d", ErrInvalidParameter, n.TemplateVersionID)
	}
	if n.ScheduledSTime == 0 {
		return fmt.Errorf("%w: ScheduledSTime = %d", ErrInvalidParameter, n.ScheduledSTime)
	}
	if n.ScheduledETime == 0 || n.ScheduledETime < n.ScheduledSTime {
		return fmt.Errorf("%w: ScheduledETime = %d", ErrInvalidParameter, n.ScheduledETime)
	}
	return nil
}

// SendNotification 发送通知
func (s *notificationService) SendNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	// 参数校验和创建通知
	n, err := s.CreateNotification(ctx, notification)
	if err != nil {
		return domain.Notification{}, err
	}

	// TODO: 实际发送逻辑，这里仅模拟发送成功

	err = s.UpdateNotificationStatus(ctx, n.ID, n.BizID, domain.StatusSucceeded)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("更新通知状态失败: %w, id = %v, bizID = %v", err, n.ID, n.BizID)
	}

	// 查询最新状态
	n.Status = domain.StatusSucceeded
	return n, nil
}

// BatchSendNotifications 批量发送通知
func (s *notificationService) BatchSendNotifications(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	if len(notifications) == 0 {
		return []domain.Notification{}, nil
	}

	// 批量参数校验
	for i := range notifications {
		if err := s.validateNotification(notifications[i]); err != nil {
			return []domain.Notification{}, err
		}
	}

	// 批量保存到仓储
	err := s.repo.BatchCreate(ctx, notifications)
	if err != nil {
		return []domain.Notification{}, fmt.Errorf("批量创建通知失败: %w", err)
	}

	// TODO: 实际批量发送逻辑，这里仅模拟所有发送成功

	var succeeded []domain.Notification
	var failed []domain.Notification

	// 批量更新状态
	err = s.repo.BatchUpdateStatusSucceededOrFailed(ctx, succeeded, failed)
	if err != nil {
		return []domain.Notification{}, fmt.Errorf("批量更新通知状态失败: %w, succeeded = %#v, failed = %#v", err, succeeded, failed)
	}

	// 获取所有通知的最新状态
	// TODO： 这里有问题，需要根据上批量发送逻辑方实现再决策
	result := make([]domain.Notification, 0, len(notifications))
	for i := range notifications {
		latestNotification, err := s.GetNotificationByID(ctx, notifications[i].ID)
		if err != nil {
			continue
		}
		result = append(result, latestNotification)
	}
	return result, nil
}

// GetNotificationByID 根据ID获取通知
func (s *notificationService) GetNotificationByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			return domain.Notification{}, ErrNotificationNotFound
		}
		return domain.Notification{}, fmt.Errorf("查询通知失败: %w", err)
	}

	return n, nil
}

// GetNotificationsByBizID 根据业务ID获取通知列表
func (s *notificationService) GetNotificationsByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error) {
	ns, err := s.repo.FindByBizID(ctx, bizID)
	if err != nil {
		return nil, fmt.Errorf("查询通知列表失败: %w", err)
	}
	return ns, nil
}

// UpdateNotificationStatus 更新通知状态
func (s *notificationService) UpdateNotificationStatus(ctx context.Context, id uint64, bizID int64, status domain.Status) error {
	err := s.repo.UpdateStatus(ctx, id, bizID, status)
	if err != nil {
		return fmt.Errorf("更新通知状态失败: %w, id = %v, bizID = %v, status = %v", err, id, bizID, status)
	}
	return nil
}

// BatchUpdateNotificationStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (s *notificationService) BatchUpdateNotificationStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	if len(succeededNotifications) == 0 && len(failedNotifications) == 0 {
		return fmt.Errorf("%w: 成功和失败的通知ID列表不能同时为空", ErrInvalidParameter)
	}

	// 批量更新状态
	err := s.repo.BatchUpdateStatusSucceededOrFailed(ctx, succeededNotifications, failedNotifications)
	if err != nil {
		return fmt.Errorf("批量更新通知状态失败: %w", err)
	}

	return nil
}
