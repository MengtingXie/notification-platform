package repository

import (
	"context"
	"errors"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/repository/dao"
)

var ErrNotificationNotFound = errors.New("通知不存在")

// NotificationRepository 通知仓储接口
type NotificationRepository interface {
	// Create 创建一条通知
	Create(ctx context.Context, n domain.Notification) error

	// BatchCreate 批量创建通知
	BatchCreate(ctx context.Context, ns []domain.Notification) error

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, bizID int64, status domain.Status) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	// FindByID 根据ID查找通知
	FindByID(ctx context.Context, id uint64) (domain.Notification, error)

	// FindByBizID 根据业务ID查找通知
	FindByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// ListByStatus 根据状态获取通知列表
	ListByStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Notification, error)

	// ListByScheduleTime 根据计划发送时间范围获取通知列表
	ListByScheduleTime(ctx context.Context, startTime, endTime time.Time, limit int) ([]domain.Notification, error)
}

// notificationRepo 通知仓储实现
type notificationRepo struct {
	dao dao.NotificationDAO
}

// NewNotificationRepository 创建通知仓储实例
func NewNotificationRepository(d dao.NotificationDAO) NotificationRepository {
	return &notificationRepo{
		dao: d,
	}
}

// Create 创建一条通知
func (r *notificationRepo) Create(ctx context.Context, n domain.Notification) error {
	return r.dao.Create(ctx, n.ToDAONotification())
}

// BatchCreate 批量创建通知
func (r *notificationRepo) BatchCreate(ctx context.Context, ns []domain.Notification) error {
	if len(ns) == 0 {
		return nil
	}

	daoNotifications := make([]dao.Notification, len(ns))
	for i := range ns {
		daoNotifications[i] = ns[i].ToDAONotification()
	}

	return r.dao.BatchCreate(ctx, daoNotifications)
}

// UpdateStatus 更新通知状态
func (r *notificationRepo) UpdateStatus(ctx context.Context, id uint64, bizID int64, status domain.Status) error {
	return r.dao.UpdateStatus(ctx, id, bizID, string(status))
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (r *notificationRepo) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	successIDs := make([]uint64, len(succeededNotifications))
	for i := range succeededNotifications {
		successIDs[i] = succeededNotifications[i].ID
	}

	// 转换失败的通知为DAO层的实体
	failedItems := make([]dao.Notification, len(failedNotifications))
	for i := range failedNotifications {
		failedItems[i] = failedNotifications[i].ToDAONotification()
	}

	return r.dao.BatchUpdateStatusSucceededOrFailed(ctx, successIDs, failedItems)
}

// FindByID 根据ID查找通知
func (r *notificationRepo) FindByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := r.dao.FindByID(ctx, id)
	if err != nil {
		return domain.Notification{}, err
	}
	return r.toDomain(n), nil
}

// toDomain 从DAO模型转换为领域模型
func (r *notificationRepo) toDomain(n dao.Notification) domain.Notification {
	return domain.Notification{
		ID:                n.ID,
		BizID:             n.BizID,
		Key:               n.Key,
		Receiver:          n.Receiver,
		Channel:           domain.Channel(n.Channel),
		TemplateID:        n.TemplateID,
		TemplateVersionID: n.TemplateVersionID,
		Status:            domain.Status(n.Status),
		RetryCount:        n.RetryCount,
		ScheduledSTime:    n.ScheduledSTime,
		ScheduledETime:    n.ScheduledETime,
	}
}

// FindByBizID 根据业务ID查找通知
func (r *notificationRepo) FindByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error) {
	ns, err := r.dao.FindByBizID(ctx, bizID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Notification, len(ns))
	for i := range ns {
		result[i] = r.toDomain(ns[i])
	}
	return result, nil
}

// ListByStatus 根据状态获取通知列表
func (r *notificationRepo) ListByStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Notification, error) {
	ns, err := r.dao.ListByStatus(ctx, string(status), limit)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Notification, len(ns))
	for i := range ns {
		result[i] = r.toDomain(ns[i])
	}
	return result, nil
}

// ListByScheduleTime 根据计划发送时间范围获取通知列表
func (r *notificationRepo) ListByScheduleTime(ctx context.Context, startTime, endTime time.Time, limit int) ([]domain.Notification, error) {
	ns, err := r.dao.ListByScheduleTime(ctx, startTime.Unix(), endTime.Unix(), limit)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Notification, len(ns))
	for i := range ns {
		result[i] = r.toDomain(ns[i])
	}
	return result, nil
}
