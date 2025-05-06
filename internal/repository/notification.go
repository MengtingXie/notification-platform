package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"github.com/gotomicro/ego/core/elog"

	"github.com/ecodeclub/ekit/slice"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
)

// NotificationRepository 通知仓储接口
type NotificationRepository interface {
	// Create 创建单条通知记录，但不创建对应的回调记录
	Create(ctx context.Context, notification domain.Notification) (domain.Notification, error)
	// CreateWithCallbackLog 创建单条通知记录，同时创建对应的回调记录
	CreateWithCallbackLog(ctx context.Context, notification domain.Notification) (domain.Notification, error)
	// BatchCreate 批量创建通知记录，但不创建对应的回调记录
	BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)
	// BatchCreateWithCallbackLog 批量创建通知记录，同时创建对应的回调记录
	BatchCreateWithCallbackLog(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)

	// GetByID 根据ID获取通知
	GetByID(ctx context.Context, id uint64) (domain.Notification, error)
	// BatchGetByIDs 根据ID列表获取通知列表
	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)

	GetByKey(ctx context.Context, bizID int64, key string) (domain.Notification, error)
	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)

	// CASStatus 更新通知状态
	CASStatus(ctx context.Context, notification domain.Notification) error
	UpdateStatus(ctx context.Context, notification domain.Notification) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error

	FindReadyNotifications(ctx context.Context, offset int, limit int) ([]domain.Notification, error)
	MarkSuccess(ctx context.Context, entity domain.Notification) error
	MarkFailed(ctx context.Context, notification domain.Notification) error
	// MarkTimeoutSendingAsFailed 将超时的 SENDING 状态的通知都标记为失败
	MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error)
}

const (
	defaultQuotaNumber int32 = 1
)

// notificationRepository 通知仓储实现
type notificationRepository struct {
	dao        dao.NotificationDAO
	quotaCache cache.QuotaCache
	logger     *elog.Component
}

// NewNotificationRepository 创建通知仓储实例
func NewNotificationRepository(d dao.NotificationDAO, quotaCache cache.QuotaCache) NotificationRepository {
	return &notificationRepository{
		dao:        d,
		quotaCache: quotaCache,
		logger:     elog.DefaultLogger,
	}
}

// Create 创建单条通知记录，但不创建对应的回调记录
func (r *notificationRepository) Create(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	// 扣减额度
	err := r.quotaCache.Decr(ctx, notification.BizID, notification.Channel, defaultQuotaNumber)
	if err != nil {
		return domain.Notification{}, err
	}
	ds, err := r.dao.Create(ctx, r.toEntity(notification))
	if err != nil {
		// 创建没成功把额度还回去
		qerr := r.quotaCache.Incr(ctx, notification.BizID, notification.Channel, defaultQuotaNumber)
		if qerr != nil {
			r.logger.Error("额度归还失败", elog.FieldErr(err),
				elog.Int64("biz_id", notification.BizID),
				elog.String("channel", notification.Channel.String()),
			)
		}
		return domain.Notification{}, err
	}
	return r.toDomain(ds), nil
}

// toEntity 将领域对象转换为DAO实体
func (r *notificationRepository) toEntity(notification domain.Notification) dao.Notification {
	templateParams, _ := notification.MarshalTemplateParams()
	receivers, _ := notification.MarshalReceivers()
	return dao.Notification{
		ID:                notification.ID,
		BizID:             notification.BizID,
		Key:               notification.Key,
		Receivers:         receivers,
		Channel:           notification.Channel.String(),
		TemplateID:        notification.Template.ID,
		TemplateVersionID: notification.Template.VersionID,
		TemplateParams:    templateParams,
		Status:            notification.Status.String(),
		ScheduledSTime:    notification.ScheduledSTime.UnixMilli(),
		ScheduledETime:    notification.ScheduledETime.UnixMilli(),
		Version:           notification.Version,
	}
}

// toDomain 将DAO实体转换为领域对象
func (r *notificationRepository) toDomain(n dao.Notification) domain.Notification {
	var templateParams map[string]string
	_ = json.Unmarshal([]byte(n.TemplateParams), &templateParams)

	var receivers []string
	_ = json.Unmarshal([]byte(n.Receivers), &receivers)

	return domain.Notification{
		ID:        n.ID,
		BizID:     n.BizID,
		Key:       n.Key,
		Receivers: receivers,
		Channel:   domain.Channel(n.Channel),
		Template: domain.Template{
			ID:        n.TemplateID,
			VersionID: n.TemplateVersionID,
			Params:    templateParams,
		},
		Status:         domain.SendStatus(n.Status),
		ScheduledSTime: time.UnixMilli(n.ScheduledSTime),
		ScheduledETime: time.UnixMilli(n.ScheduledETime),
		Version:        n.Version,
	}
}

// CreateWithCallbackLog 创建单条通知记录，同时创建对应的回调记录
func (r *notificationRepository) CreateWithCallbackLog(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	// 扣减额度
	err := r.quotaCache.Decr(ctx, notification.BizID, notification.Channel, defaultQuotaNumber)
	if err != nil {
		return domain.Notification{}, err
	}
	ds, err := r.dao.CreateWithCallbackLog(ctx, r.toEntity(notification))
	if err != nil {
		qerr := r.quotaCache.Incr(ctx, notification.BizID, notification.Channel, defaultQuotaNumber)
		if qerr != nil {
			r.logger.Error("额度归还失败", elog.FieldErr(err),
				elog.Int64("biz_id", notification.BizID),
				elog.String("channel", notification.Channel.String()),
			)
		}
		return domain.Notification{}, err
	}
	return r.toDomain(ds), nil
}

// BatchCreate 批量创建通知记录，但不创建对应的回调记录
func (r *notificationRepository) BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	return r.batchCreate(ctx, notifications, false)
}

func (r *notificationRepository) batchCreate(ctx context.Context, notifications []domain.Notification, createCallbackLog bool) ([]domain.Notification, error) {
	if len(notifications) == 0 {
		return nil, nil
	}

	daoNotifications := slice.Map(notifications, func(_ int, src domain.Notification) dao.Notification {
		return r.toEntity(src)
	})

	var createdNotifications []dao.Notification
	var err error
	// 扣减库存
	err = r.mutiDecr(ctx, notifications)
	if err != nil {
		return nil, err
	}
	if createCallbackLog {
		createdNotifications, err = r.dao.BatchCreateWithCallbackLog(ctx, daoNotifications)
		if err != nil {
			eerr := r.mutiIncr(ctx, notifications)
			if eerr != nil {
				elog.Error("发送失败，归还额度失败", elog.FieldErr(eerr))
			}
			return nil, err
		}
	} else {
		createdNotifications, err = r.dao.BatchCreate(ctx, daoNotifications)
		if err != nil {
			eerr := r.mutiIncr(ctx, notifications)
			if eerr != nil {
				elog.Error("发送失败，归还额度失败", elog.FieldErr(eerr))
			}
			return nil, err
		}
	}

	return slice.Map(createdNotifications, func(_ int, src dao.Notification) domain.Notification {
		return r.toDomain(src)
	}), nil
}

func (r *notificationRepository) mutiDecr(ctx context.Context, notifications []domain.Notification) error {
	return r.quotaCache.MutiDecr(ctx, r.getItems(notifications))
}

func (r *notificationRepository) mutiIncr(ctx context.Context, notifications []domain.Notification) error {
	return r.quotaCache.MutiIncr(ctx, r.getItems(notifications))
}

func (r *notificationRepository) getItems(notifications []domain.Notification) []cache.IncrItem {
	notiMap := make(map[string]cache.IncrItem)
	for idx := range notifications {
		d := notifications[idx]
		key := fmt.Sprintf("%d-%s", d.BizID, d.Channel.String())
		item, ok := notiMap[key]
		if !ok {
			item = cache.IncrItem{
				BizID:   d.BizID,
				Channel: d.Channel,
			}
		}
		item.Val++
		notiMap[key] = item
	}
	items := make([]cache.IncrItem, 0, len(notiMap))
	for key := range notiMap {
		items = append(items, notiMap[key])
	}
	return items
}

// BatchCreateWithCallbackLog 批量创建通知记录，同时创建对应的回调记录
func (r *notificationRepository) BatchCreateWithCallbackLog(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	return r.batchCreate(ctx, notifications, true)
}

// GetByID 根据ID获取通知
func (r *notificationRepository) GetByID(ctx context.Context, id uint64) (domain.Notification, error) {
	n, err := r.dao.GetByID(ctx, id)
	if err != nil {
		return domain.Notification{}, err
	}
	return r.toDomain(n), nil
}

func (r *notificationRepository) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error) {
	//
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

func (r *notificationRepository) GetByKey(ctx context.Context, bizID int64, key string) (domain.Notification, error) {
	not, err := r.dao.GetByKey(ctx, bizID, key)
	return r.toDomain(not), err
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

// CASStatus 更新通知状态
func (r *notificationRepository) CASStatus(ctx context.Context, notification domain.Notification) error {
	return r.dao.CASStatus(ctx, r.toEntity(notification))
}

func (r *notificationRepository) UpdateStatus(ctx context.Context, notification domain.Notification) error {
	return r.dao.UpdateStatus(ctx, r.toEntity(notification))
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
func (r *notificationRepository) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	// 转换成功的通知为DAO层的实体
	successItems := make([]dao.Notification, len(succeededNotifications))
	for i := range succeededNotifications {
		successItems[i] = r.toEntity(succeededNotifications[i])
	}

	// 转换失败的通知为DAO层的实体
	failedItems := make([]dao.Notification, len(failedNotifications))
	for i := range failedNotifications {
		failedItems[i] = r.toEntity(failedNotifications[i])
	}

	err := r.dao.BatchUpdateStatusSucceededOrFailed(ctx, successItems, failedItems)
	if err != nil {
		return err
	}

	items := r.getItems(failedNotifications)
	eerr := r.quotaCache.MutiIncr(ctx, items)
	if eerr != nil {
		elog.Error("发送失败，归还额度失败", elog.FieldErr(eerr))
	}
	return nil
}

func (r *notificationRepository) FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error) {
	nos, err := r.dao.FindReadyNotifications(ctx, offset, limit)
	return slice.Map(nos, func(_ int, src dao.Notification) domain.Notification {
		return r.toDomain(src)
	}), err
}

func (r *notificationRepository) MarkSuccess(ctx context.Context, notification domain.Notification) error {
	return r.dao.MarkSuccess(ctx, r.toEntity(notification))
}

func (r *notificationRepository) MarkFailed(ctx context.Context, notification domain.Notification) error {
	return r.dao.MarkFailed(ctx, r.toEntity(notification))
}

func (r *notificationRepository) MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error) {
	return r.dao.MarkTimeoutSendingAsFailed(ctx, batchSize)
}
