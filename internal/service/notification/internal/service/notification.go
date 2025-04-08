package service

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/repository"

	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"github.com/sony/sonyflake"
)

var (
	ErrInvalidParameter             = errors.New("参数非法")
	ErrNotificationIDGenerateFailed = errors.New("通知ID生成失败")
	ErrNotificationNotFound         = repository.ErrNotificationNotFound
	ErrCreateNotificationFailed     = errors.New("创建通知失败")
	ErrNotificationDuplicate        = repository.ErrNotificationDuplicate
)

// NotificationService 通知服务接口
//
//go:generate mockgen -source=./notification.go -destination=../../mocks/notification.mock.go -package=notificationmocks -typed NotificationService
type NotificationService interface {
	// Create 创建通知记录
	Create(ctx context.Context, notification domain.Notification) (domain.Notification, error)

	// BatchCreate 批量创建通知记录
	BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)

	// GetByID 根据ID获取通知记录
	GetByID(ctx context.Context, id uint64) (domain.Notification, error)

	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)

	// GetByBizID 根据业务ID获取通知记录列表
	GetByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, status domain.Status, version int) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.Status) error
}

// notificationService 通知服务实现
type notificationService struct {
	repo        repository.NotificationRepository
	idGenerator *sonyflake.Sonyflake
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(repo repository.NotificationRepository, idGenerator *sonyflake.Sonyflake) NotificationService {
	return &notificationService{
		repo:        repo,
		idGenerator: idGenerator,
	}
}

// Create 创建通知
func (s *notificationService) Create(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	if err := s.validateNotification(notification); err != nil {
		return domain.Notification{}, fmt.Errorf("%w: %w", ErrInvalidParameter, err)
	}

	// 生成ID
	id, err := s.idGenerator.NextID()
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%w", ErrNotificationIDGenerateFailed)
	}
	notification.ID = id

	createdNotification, err := s.repo.Create(ctx, notification)
	if err != nil {
		if errors.Is(err, ErrNotificationDuplicate) {
			return domain.Notification{}, fmt.Errorf("%w", ErrNotificationDuplicate)
		}
		return domain.Notification{}, fmt.Errorf("创建通知失败: %w", err)
	}

	return createdNotification, nil
}

// validateNotification 验证通知参数
func (s *notificationService) validateNotification(n domain.Notification) error {
	if n.Key == "" {
		return fmt.Errorf("%w: Key = %q", ErrInvalidParameter, n.Key)
	}
	if n.Receiver == "" {
		return fmt.Errorf("%w: Receiver = %q", ErrInvalidParameter, n.Receiver)
	}
	if n.Channel == "" {
		return fmt.Errorf("%w: Channel = %q", ErrInvalidParameter, n.Channel)
	}
	if n.Template.ID <= 0 {
		return fmt.Errorf("%w: Template.ID = %d", ErrInvalidParameter, n.Template.ID)
	}
	if n.Template.VersionID <= 0 {
		return fmt.Errorf("%w: Template.VersionID = %d", ErrInvalidParameter, n.Template.VersionID)
	}
	if len(n.Template.Params) == 0 {
		return fmt.Errorf("%w: Template.Params = %q", ErrInvalidParameter, n.Template.Params)
	}
	if n.ScheduledSTime == 0 {
		return fmt.Errorf("%w: ScheduledSTime = %d", ErrInvalidParameter, n.ScheduledSTime)
	}
	if n.ScheduledETime == 0 || n.ScheduledETime < n.ScheduledSTime {
		return fmt.Errorf("%w: ScheduledETime = %d", ErrInvalidParameter, n.ScheduledETime)
	}
	return nil
}

// BatchCreate 批量创建通知记录
func (s *notificationService) BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	if len(notifications) == 0 {
		return nil, fmt.Errorf("%w: 通知列表为空", ErrInvalidParameter)
	}

	for i := range notifications {
		if err := s.validateNotification(notifications[i]); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidParameter, err)
		}
	}

	// 生成ID
	for i := range notifications {
		id, err := s.idGenerator.NextID()
		if err != nil {
			return nil, fmt.Errorf("%w", ErrNotificationIDGenerateFailed)
		}
		notifications[i].ID = id
	}

	createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
	if err != nil {
		if errors.Is(err, ErrNotificationDuplicate) {
			return nil, fmt.Errorf("%w", ErrNotificationDuplicate)
		}
		return nil, fmt.Errorf("批量创建通知失败: %w", err)
	}

	return createdNotifications, nil
}

// GetByID 获取通知记录
func (s *notificationService) GetByID(ctx context.Context, id uint64) (domain.Notification, error) {
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			return domain.Notification{}, fmt.Errorf("%w: id=%d", ErrNotificationNotFound, id)
		}
		return domain.Notification{}, fmt.Errorf("获取通知失败: %w", err)
	}
	return notification, nil
}

func (s *notificationService) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error) {
	return s.repo.BatchGetByIDs(ctx, ids)
}

// GetByBizID 根据业务ID获取通知记录列表
func (s *notificationService) GetByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error) {
	notifications, err := s.repo.GetByBizID(ctx, bizID)
	if err != nil {
		return nil, fmt.Errorf("获取通知列表失败: %w", err)
	}
	return notifications, nil
}

// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
func (s *notificationService) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: 业务内唯一标识列表空", ErrInvalidParameter)
	}

	notifications, err := s.repo.GetByKeys(ctx, bizID, keys...)
	if err != nil {
		return nil, fmt.Errorf("获取通知列表失败: %w", err)
	}
	return notifications, nil
}

// UpdateStatus 更新通知状态
func (s *notificationService) UpdateStatus(ctx context.Context, id uint64, status domain.Status, version int) error {
	err := s.repo.UpdateStatus(ctx, id, status, version)
	if err != nil {
		return fmt.Errorf("更新通知状态失败: %w", err)
	}
	return nil
}

func (s *notificationService) BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.Status) error {
	return s.repo.BatchUpdateStatus(ctx, ids, status)
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (s *notificationService) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
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
