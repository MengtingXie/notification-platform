package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/repository/dao"
)

var (
	ErrNotificationNotFound  = dao.ErrNotificationNotFound
	ErrNotificationDuplicate = dao.ErrNotificationDuplicate
)

// NotificationRepository 通知仓储接口
type NotificationRepository interface {
	// Create 创建一条通知
	Create(ctx context.Context, notification domain.Notification) (domain.Notification, error)

	// BatchCreate 批量创建通知
	BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)

	// GetByID 根据ID获取通知
	GetByID(ctx context.Context, id uint64) (domain.Notification, error)

	// BatchGetByIDs 根据ID列表获取通知列表
	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)

	// GetByBizID 根据业务ID获取通知列表
	GetByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, status domain.Status, version int) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	// BatchUpdateStatus 批量更新通知状态
	BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.Status) error

	// ListByStatus 根据状态获取通知列表
	ListByStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Notification, error)

	// ListByScheduleTime 根据计划发送时间范围获取通知列表
	ListByScheduleTime(ctx context.Context, startTime, endTime time.Time, limit int) ([]domain.Notification, error)
}

// notificationRepository 通知仓储实现
type notificationRepository struct {
	dao dao.NotificationDAO
}

// NewNotificationRepository 创建通知仓储实例
func NewNotificationRepository(d dao.NotificationDAO) NotificationRepository {
	return &notificationRepository{
		dao: d,
	}
}

func (r *notificationRepository) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error) {
	notificationMap, err := r.dao.BatchGetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	domainNotificationMap := make(map[uint64]domain.Notification, len(notificationMap))
	for id := range notificationMap {
		notification := notificationMap[id]
		domainNotificationMap[id] = r.toDomain(notification)
	}
	return domainNotificationMap, nil
}

// Create 创建一条通知
func (r *notificationRepository) Create(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	ds, err := r.dao.Create(ctx, r.toEntity(notification))
	if err != nil {
		return domain.Notification{}, err
	}
	return r.toDomain(ds), nil
}

// toEntity 将领域对象转换为DAO实体
func (r *notificationRepository) toEntity(n domain.Notification) dao.Notification {
	templateParams, _ := json.Marshal(n.Template.Params)

	return dao.Notification{
		ID:                n.ID,
		BizID:             n.BizID,
		Key:               n.Key,
		Receiver:          n.Receiver,
		Channel:           string(n.Channel),
		TemplateID:        n.Template.ID,
		TemplateVersionID: n.Template.VersionID,
		TemplateParams:    string(templateParams),
		Status:            string(n.Status),
		RetryCount:        n.RetryCount,
		ScheduledSTime:    n.ScheduledSTime,
		ScheduledETime:    n.ScheduledETime,
		Version:           n.Version,
	}
}

// BatchCreate 批量创建通知
func (r *notificationRepository) BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	if len(notifications) == 0 {
		return nil, nil
	}

	daoNotifications := make([]dao.Notification, len(notifications))
	for i := range notifications {
		daoNotifications[i] = r.toEntity(notifications[i])
	}

	createdNotifications, err := r.dao.BatchCreate(ctx, daoNotifications)
	if err != nil {
		return nil, err
	}

	for i := range createdNotifications {
		notifications[i] = r.toDomain(createdNotifications[i])
	}
	return notifications, nil
}

// UpdateStatus 更新通知状态
func (r *notificationRepository) UpdateStatus(ctx context.Context, id uint64, status domain.Status, version int) error {
	return r.dao.UpdateStatus(ctx, id, string(status), version)
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (r *notificationRepository) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	successIDs := make([]uint64, len(succeededNotifications))
	for i := range succeededNotifications {
		successIDs[i] = succeededNotifications[i].ID
	}

	// 转换失败的通知为DAO层的实体
	failedItems := make([]dao.Notification, len(failedNotifications))
	for i := range failedNotifications {
		failedItems[i] = r.toEntity(failedNotifications[i])
	}

	return r.dao.BatchUpdateStatusSucceededOrFailed(ctx, successIDs, failedItems)
}

// GetByID 根据ID获取通知
func (r *notificationRepository) GetByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := r.dao.GetByID(ctx, id)
	if err != nil {
		return domain.Notification{}, err
	}
	return r.toDomain(n), nil
}

// toDomain 将DAO实体转换为领域对象
func (r *notificationRepository) toDomain(n dao.Notification) domain.Notification {
	var templateParams map[string]string
	_ = json.Unmarshal([]byte(n.TemplateParams), &templateParams)

	return domain.Notification{
		ID:       n.ID,
		BizID:    n.BizID,
		Key:      n.Key,
		Receiver: n.Receiver,
		Channel:  domain.Channel(n.Channel),
		Template: domain.Template{
			ID:        n.TemplateID,
			VersionID: n.TemplateVersionID,
			Params:    templateParams,
		},
		Status:         domain.Status(n.Status),
		RetryCount:     n.RetryCount,
		ScheduledSTime: n.ScheduledSTime,
		ScheduledETime: n.ScheduledETime,
		Version:        n.Version,
	}
}

// GetByBizID 根据业务ID获取通知列表
func (r *notificationRepository) GetByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error) {
	ns, err := r.dao.GetByBizID(ctx, bizID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Notification, len(ns))
	for i := range ns {
		result[i] = r.toDomain(ns[i])
	}
	return result, nil
}

// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
func (r *notificationRepository) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	notifications, err := r.dao.GetByKeys(ctx, bizID, keys...)
	if err != nil {
		return nil, fmt.Errorf("查询通知列表失败: %w", err)
	}
	result := make([]domain.Notification, len(notifications))
	for i := range notifications {
		result[i] = r.toDomain(notifications[i])
	}
	return result, nil
}

// ListByStatus 根据状态获取通知列表
func (r *notificationRepository) ListByStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Notification, error) {
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
func (r *notificationRepository) ListByScheduleTime(ctx context.Context, startTime, endTime time.Time, limit int) ([]domain.Notification, error) {
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

func (r *notificationRepository) BatchUpdateStatus(ctx context.Context, ids []uint64, status domain.Status) error {
	return r.dao.BatchUpdateStatus(ctx, ids, string(status))
}
