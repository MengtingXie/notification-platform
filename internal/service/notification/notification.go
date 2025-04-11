package notification

import (
	"context"
	"errors"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/errs"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"

	"github.com/sony/sonyflake"
)

// Service 通知服务接口
//
//go:generate mockgen -source=./notification.go -destination=../../mocks/notification.mock.go -package=notificationmocks -typed Service
type Service interface {
	// FindReadyNotifications 准备好调度发送的通知
	FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error)

	// GetByID 根据ID获取通知记录
	GetByID(ctx context.Context, id uint64) (domain.Notification, error)

	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)

	// GetByBizID 根据业务ID获取通知记录列表
	GetByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, notification domain.Notification) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.SendStatus) error
}

// notificationService 通知服务实现
type notificationService struct {
	repo        repository.NotificationRepository
	idGenerator *sonyflake.Sonyflake
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(repo repository.NotificationRepository, idGenerator *sonyflake.Sonyflake) Service {
	return &notificationService{
		repo:        repo,
		idGenerator: idGenerator,
	}
}

func (s *notificationService) FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error) {
	return s.repo.FindReadyNotifications(ctx, offset, limit)
}

// GetByID 获取通知记录
func (s *notificationService) GetByID(ctx context.Context, id uint64) (domain.Notification, error) {
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, errs.ErrNotificationNotFound) {
			return domain.Notification{}, fmt.Errorf("%w: id=%d", errs.ErrNotificationNotFound, id)
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
		return nil, fmt.Errorf("%w: 业务内唯一标识列表空", errs.ErrInvalidParameter)
	}

	notifications, err := s.repo.GetByKeys(ctx, bizID, keys...)
	if err != nil {
		return nil, fmt.Errorf("获取通知列表失败: %w", err)
	}
	return notifications, nil
}

// UpdateStatus 更新通知状态
func (s *notificationService) UpdateStatus(ctx context.Context, notification domain.Notification) error {
	err := s.repo.CASStatus(ctx, notification)
	if err != nil {
		return fmt.Errorf("更新通知状态失败: %w", err)
	}
	return nil
}

func (s *notificationService) BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.SendStatus) error {
	return s.repo.BatchUpdateStatus(ctx, ids, status)
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (s *notificationService) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	if len(succeededNotifications) == 0 && len(failedNotifications) == 0 {
		return fmt.Errorf("%w: 成功和失败的通知ID列表不能同时为空", errs.ErrInvalidParameter)
	}

	// 批量更新状态
	err := s.repo.BatchUpdateStatusSucceededOrFailed(ctx, succeededNotifications, failedNotifications)
	if err != nil {
		return fmt.Errorf("批量更新通知状态失败: %w", err)
	}

	return nil
}
