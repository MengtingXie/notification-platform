package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/repository/dao"
)

var ErrNotificationNotFound = dao.ErrNotificationNotFound

// NotificationRepository 通知仓储接口
type NotificationRepository interface {
	// Create 创建一条通知
	Create(ctx context.Context, n domain.Notification) error

	// BatchCreate 批量创建通知
	BatchCreate(ctx context.Context, ns []domain.Notification) error

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, status domain.Status) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	// FindByID 根据ID查找通知
	FindByID(ctx context.Context, id uint64) (domain.Notification, error)

	// FindByBizID 根据业务ID查找通知
	FindByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error)

	// FindByKeys 根据业务ID和业务内唯一标识获取通知列表
	FindByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

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

// Create 创建一条通知
func (r *notificationRepository) Create(ctx context.Context, n domain.Notification) error {
	return r.dao.Create(ctx, r.toEntity(n))
}

// toEntity 将领域模型转换为DAO模型
func (r *notificationRepository) toEntity(n domain.Notification) dao.Notification {
	// 将 TemplateParams 转换为 JSON 字符串
	templateParamsJSON, _ := json.Marshal(n.Template.Params)

	return dao.Notification{
		ID:                n.ID,
		BizID:             n.BizID,
		Key:               n.Key,
		Receiver:          n.Receiver,
		Channel:           string(n.Channel),
		TemplateID:        n.Template.ID,
		TemplateVersionID: n.Template.VersionID,
		TemplateParams:    string(templateParamsJSON),
		Status:            string(n.Status),
		RetryCount:        n.RetryCount,
		ScheduledSTime:    n.ScheduledSTime,
		ScheduledETime:    n.ScheduledETime,
	}
}

// BatchCreate 批量创建通知
func (r *notificationRepository) BatchCreate(ctx context.Context, ns []domain.Notification) error {
	if len(ns) == 0 {
		return nil
	}

	daoNotifications := make([]dao.Notification, len(ns))
	for i := range ns {
		daoNotifications[i] = r.toEntity(ns[i])
	}

	return r.dao.BatchCreate(ctx, daoNotifications)
}

// UpdateStatus 更新通知状态
func (r *notificationRepository) UpdateStatus(ctx context.Context, id uint64, status domain.Status) error {
	return r.dao.UpdateStatus(ctx, id, string(status))
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

// FindByID 根据ID查找通知
func (r *notificationRepository) FindByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := r.dao.FindByID(ctx, id)
	if err != nil {
		return domain.Notification{}, err
	}
	return r.toDomain(n), nil
}

// toDomain 从DAO模型转换为领域模型
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
	}
}

// FindByBizID 根据业务ID查找通知
func (r *notificationRepository) FindByBizID(ctx context.Context, bizID int64) ([]domain.Notification, error) {
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

// FindByKeys 根据业务ID和业务内唯一标识获取通知列表
func (r *notificationRepository) FindByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	notifications, err := r.dao.FindByKeys(ctx, bizID, keys...)
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
