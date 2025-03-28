package service

import (
	"context"
	"errors"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/repository"
	"github.com/sony/sonyflake"
)

var (
	ErrInvalidParameter             = errors.New("参数非法")
	ErrNotificationIDGenerateFailed = errors.New("通知ID生成失败")
	ErrNotificationNotFound         = repository.ErrNotificationNotFound
	ErrChannelDisabled              = errors.New("通知渠道已禁用")
)

// NotificationService 通知服务接口
//
//go:generate mockgen -source=./notification.go -destination=../../mocks/notification.mock.go -package=notificationmocks -typed NotificationService
type NotificationService interface {
	// CreateNotification 创建通知记录
	CreateNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error)

	// BatchCreateNotifications 批量创建通知记录
	BatchCreateNotifications(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)

	// GetNotificationByID 根据ID获取通知记录
	GetNotificationByID(ctx context.Context, id uint64) (domain.Notification, error)

	// GetNotificationsByBizID 根据业务ID获取通知记录列表
	GetNotificationsByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// GetNotificationsByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetNotificationsByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

	// UpdateNotificationStatus 更新通知状态
	UpdateNotificationStatus(ctx context.Context, id uint64, status domain.Status) error

	// BatchUpdateNotificationStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateNotificationStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	BatchUpdateNotificationStatus(ctx context.Context, ids []uint64, status string) error

	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)
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

func (s *notificationService) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error) {
	return s.repo.BatchGetByIDs(ctx, ids)
}

func (s *notificationService) BatchUpdateNotificationStatus(ctx context.Context, ids []uint64, status string) error {
	return s.repo.BatchUpdateStatus(ctx, ids, status)
}

// CreateNotification 创建通知
func (s *notificationService) CreateNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	if err := s.validateNotification(notification); err != nil {
		return domain.Notification{}, err
	}

	id, err := s.idGenerator.NextID()
	if err != nil {
		return domain.Notification{}, ErrNotificationIDGenerateFailed
	}
	notification.ID = id

	err2 := s.repo.Create(ctx, notification)
	if err2 != nil {
		return domain.Notification{}, fmt.Errorf("创建通知记录失败: %w", err2)
	}
	notification.ID = id

	return notification, nil
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

// BatchCreateNotifications 批量创建通知记录
func (s *notificationService) BatchCreateNotifications(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	// 验证所有通知
	if len(notifications) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	for i := range notifications {
		if err := s.validateNotification(notifications[i]); err != nil {
			return nil, err
		}
	}

	// 生成ID
	for i := range notifications {
		id, err := s.idGenerator.NextID()
		if err != nil {
			return nil, ErrNotificationIDGenerateFailed
		}
		notifications[i].ID = id
	}

	err := s.repo.BatchCreate(ctx, notifications)
	if err != nil {
		return nil, fmt.Errorf("批量创建通知记录失败: %w", err)
	}

	return notifications, nil
}

// GetNotificationByID 根据ID获取通知
func (s *notificationService) GetNotificationByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			return domain.Notification{}, fmt.Errorf("%w", ErrNotificationNotFound)
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

// GetNotificationsByKeys 根据业务ID和业务内唯一标识获取通知列表
func (s *notificationService) GetNotificationsByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: 业务内唯一标识列表不能为空", ErrInvalidParameter)
	}

	ns, err := s.repo.FindByKeys(ctx, bizID, keys...)
	if err != nil {
		return nil, fmt.Errorf("查询通知列表失败: %w", err)
	}
	return ns, nil
}

// UpdateNotificationStatus 更新通知状态
func (s *notificationService) UpdateNotificationStatus(ctx context.Context, id uint64, status domain.Status) error {
	err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return fmt.Errorf("更新通知状态失败: %w, id = %v, status = %v", err, id, status)
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
