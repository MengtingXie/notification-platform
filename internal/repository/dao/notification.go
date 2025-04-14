package dao

import (
	"context"
	"errors"
	"fmt"
	"github.com/ecodeclub/ekit/slice"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"

	"github.com/ego-component/egorm"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type NotificationDAO interface {
	// Create 创建单条通知记录，但不创建对应的回调记录
	Create(ctx context.Context, data Notification) (Notification, error)
	// CreateWithCallbackLog 创建单条通知记录，同时创建对应的回调记录
	CreateWithCallbackLog(ctx context.Context, data Notification) (Notification, error)
	// BatchCreate 批量创建通知记录，但不创建对应的回调记录
	BatchCreate(ctx context.Context, dataList []Notification) ([]Notification, error)
	// BatchCreateWithCallbackLog 批量创建通知记录，同时创建对应的回调记录
	BatchCreateWithCallbackLog(ctx context.Context, datas []Notification) ([]Notification, error)

	// GetByID 根据ID查询通知
	GetByID(ctx context.Context, id uint64) (Notification, error)

	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]Notification, error)

	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]Notification, error)
	// GetByKey 根据业务ID和业务内唯一标识获取通知列表
	GetByKey(ctx context.Context, bizID int64, key string) (Notification, error)

	// CASStatus 更新通知状态
	CASStatus(ctx context.Context, notification Notification) error
	UpdateStatus(ctx context.Context, notification Notification) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败，使用乐观锁控制并发
	// successNotifications: 更新为成功状态的通知列表，包含ID、Version和重试次数
	// failedNotifications: 更新为失败状态的通知列表，包含ID、Version和重试次数
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, successNotifications, failedNotifications []Notification) error

	FindReadyNotifications(ctx context.Context, offset, limit int) ([]Notification, error)

	MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error)
	MarkFailed(ctx context.Context, entity Notification) error
	MarkSuccess(ctx context.Context, entity Notification) error
}

// Notification 通知记录表
type Notification struct {
	ID                uint64 `gorm:"primaryKey;comment:'雪花算法ID'"`
	BizID             int64  `gorm:"type:BIGINT;NOT NULL;index:idx_biz_id_status,priority:1;uniqueIndex:idx_biz_id_key,priority:1;comment:'业务配表ID，业务方可能有多个业务每个业务配置不同'"`
	Key               string `gorm:"type:VARCHAR(256);NOT NULL;uniqueIndex:idx_biz_id_key,priority:2;comment:'业务内唯一标识，区分同一个业务内的不同通知'"`
	Receivers         string `gorm:"type:TEXT;NOT NULL;comment:'接收者(手机/邮箱/用户ID)，JSON数组'"`
	Channel           string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;comment:'发送渠道'"`
	TemplateID        int64  `gorm:"type:BIGINT;NOT NULL;comment:'模板ID'"`
	TemplateVersionID int64  `gorm:"type:BIGINT;NOT NULL;comment:'模板版本ID'"`
	TemplateParams    string `gorm:"NOT NULL;comment:'模版参数'"`
	Status            string `gorm:"type:ENUM('PREPARE','CANCELED','PENDING','SENDING','SUCCEEDED','FAILED');DEFAULT:'PENDING';index:idx_biz_id_status,priority:2;index:idx_scheduled,priority:3;comment:'发送状态'"`
	ScheduledSTime    int64  `gorm:"column:scheduled_stime;index:idx_scheduled,priority:1;comment:'计划发送开始时间'"`
	ScheduledETime    int64  `gorm:"column:scheduled_etime;index:idx_scheduled,priority:2;comment:'计划发送结束时间'"`
	Version           int    `gorm:"type:INT;NOT NULL;DEFAULT:1;comment:'版本号，用于CAS操作'"`
	Ctime             int64
	Utime             int64
}

type notificationDAO struct {
	db *egorm.Component
}

// NewNotificationDAO 创建通知DAO实例
func NewNotificationDAO(db *egorm.Component) NotificationDAO {
	return &notificationDAO{
		db: db,
	}
}

// Create 创建单条通知记录，但不创建对应的回调记录
func (d *notificationDAO) Create(ctx context.Context, data Notification) (Notification, error) {
	return d.create(ctx, d.db, data, false)
}

// CreateWithCallbackLog 创建单条通知记录，同时创建对应的回调记录
func (d *notificationDAO) CreateWithCallbackLog(ctx context.Context, data Notification) (Notification, error) {
	return d.create(ctx, d.db, data, true)
}

// create 创建通知记录，以及可能的对应回调记录
func (d *notificationDAO) create(ctx context.Context, db *gorm.DB, data Notification, createCallbackLog bool) (Notification, error) {
	now := time.Now().UnixMilli()
	data.Ctime, data.Utime = now, now
	data.Version = 1

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&data).Error; err != nil {
			if d.isUniqueConstraintError(err) {
				return fmt.Errorf("%w", errs.ErrNotificationDuplicate)
			}
			return err
		}
		res := tx.Model(&Quota{}).Where("quota >=1 AND biz_id = ? AND channel = ? ",
			data.BizID, data.Channel).Updates(
			map[string]any{
				"quota": gorm.Expr("`quota`-1"),
				"utime": now,
			})
		if res.Error != nil || res.RowsAffected == 0 {
			return fmt.Errorf("%w，原因: %w", errs.ErrNoQuota, res.Error)
		}

		// 直接数据库操作，直接扣减，CAS 扣减1
		if createCallbackLog {
			if err := tx.Create(&CallbackLog{
				NotificationID: data.ID,
				Status:         domain.CallbackLogStatusInit.String(),
				NextRetryTime:  now}).Error; err != nil {
				return fmt.Errorf("%w", errs.ErrCreateCallbackLogFailed)
			}
		}
		return nil
	})

	return data, err
}

// isUniqueConstraintError 检查是否是唯一索引冲突错误
func (d *notificationDAO) isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	me := new(mysql.MySQLError)
	if ok := errors.As(err, &me); ok {
		const uniqueIndexErrNo uint16 = 1062
		return me.Number == uniqueIndexErrNo
	}
	return false
}

// BatchCreate 批量创建通知记录，但不创建对应的回调记录
func (d *notificationDAO) BatchCreate(ctx context.Context, datas []Notification) ([]Notification, error) {
	return d.batchCreate(ctx, datas, false)
}

// BatchCreateWithCallbackLog 批量创建通知记录，同时创建对应的回调记录
func (d *notificationDAO) BatchCreateWithCallbackLog(ctx context.Context, datas []Notification) ([]Notification, error) {
	return d.batchCreate(ctx, datas, true)
}

// batchCreate 批量创建通知记录，以及可能的对应回调记录
func (d *notificationDAO) batchCreate(ctx context.Context, datas []Notification, createCallbackLog bool) ([]Notification, error) {
	if len(datas) == 0 {
		return []Notification{}, nil
	}

	const batchSize = 100
	now := time.Now().UnixMilli()
	for i := range datas {
		datas[i].Ctime, datas[i].Utime = now, now
		datas[i].Version = 1
	}

	// 使用事务执行批量插入
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建通知记录 - 真正的批量插入
		if err := tx.CreateInBatches(datas, batchSize).Error; err != nil {
			if d.isUniqueConstraintError(err) {
				return fmt.Errorf("%w", errs.ErrNotificationDuplicate)
			}
			return err
		}

		if createCallbackLog {
			// 创建回调记录
			var callbackLogs []CallbackLog
			for i := range datas {
				callbackLogs = append(callbackLogs, CallbackLog{
					NotificationID: datas[i].ID,
					NextRetryTime:  now,
					Ctime:          now,
					Utime:          now,
				})
			}
			if err := tx.CreateInBatches(callbackLogs, batchSize).Error; err != nil {
				return fmt.Errorf("%w", errs.ErrCreateCallbackLogFailed)
			}
		}
		return nil
	})

	return datas, err
}

// GetByID 根据ID查询通知
func (d *notificationDAO) GetByID(ctx context.Context, id uint64) (Notification, error) {
	var notification Notification
	err := d.db.WithContext(ctx).First(&notification, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Notification{}, fmt.Errorf("%w: id=%d", errs.ErrNotificationNotFound, id)
		}
		return Notification{}, err
	}
	return notification, nil
}

func (d *notificationDAO) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]Notification, error) {
	var notifications []Notification
	err := d.db.WithContext(ctx).
		Where("id in (?)", ids).
		Find(&notifications).Error
	notificationMap := make(map[uint64]Notification, len(ids))
	for idx := range notifications {
		notification := notifications[idx]
		notificationMap[notification.ID] = notification
	}
	return notificationMap, err
}

// GetByBizID 根据业务ID查询通知列表
func (d *notificationDAO) GetByBizID(ctx context.Context, bizID int64) ([]Notification, error) {
	var notifications []Notification
	err := d.db.WithContext(ctx).Where("biz_id = ?", bizID).Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func (d *notificationDAO) GetByKey(ctx context.Context, bizID int64, key string) (Notification, error) {
	var not Notification
	err := d.db.WithContext(ctx).Where("biz_id = ? AND `key` = ?", bizID, key).First(&not).Error
	if err != nil {
		return Notification{}, fmt.Errorf("查询通知列表失败:bizID: %d, key %s %w", bizID, key, err)
	}
	return not, nil
}

// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
func (d *notificationDAO) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]Notification, error) {
	var notifications []Notification
	err := d.db.WithContext(ctx).Where("biz_id = ? AND `key` IN ?", bizID, keys).Find(&notifications).Error
	if err != nil {
		return nil, fmt.Errorf("查询通知列表失败: %w", err)
	}
	return notifications, nil
}

// CASStatus 更新通知状态
func (d *notificationDAO) CASStatus(ctx context.Context, notification Notification) error {
	updates := map[string]any{
		"status":  notification.Status,
		"version": gorm.Expr("version + 1"),
		"utime":   time.Now().Unix(),
	}

	result := d.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND version = ?", notification.ID, notification.Version).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected < 1 {
		return fmt.Errorf("并发竞争失败 %w, id %d", errs.ErrNotificationVersionMismatch, notification.ID)
	}
	return nil
}

func (d *notificationDAO) UpdateStatus(ctx context.Context, notification Notification) error {
	return d.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ?", notification.ID).
		Updates(map[string]any{
			"status":  notification.Status,
			"version": gorm.Expr("version + 1"),
			"utime":   time.Now().Unix(),
		}).Error
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败，使用乐观锁控制并发
// successNotifications: 更新为成功状态的通知列表，包含ID、Version和重试次数
// failedNotifications: 更新为失败状态的通知列表，包含ID、Version和重试次数
func (d *notificationDAO) BatchUpdateStatusSucceededOrFailed(ctx context.Context, successNotifications, failedNotifications []Notification) error {
	if len(successNotifications) == 0 && len(failedNotifications) == 0 {
		return nil
	}

	successIds := slice.Map(successNotifications, func(idx int, src Notification) uint64 {
		return src.ID
	})

	failedIds := slice.Map(failedNotifications, func(idx int, src Notification) uint64 {
		return src.ID
	})

	// 开启事务
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if len(successIds) != 0 {
			err := d.batchMarkSuccess(tx, successIds)
			if err != nil {
				return err
			}
		}

		if len(failedIds) != 0 {
			now := time.Now().Unix()
			return tx.Model(&Notification{}).
				Where("id IN ?", failedIds).
				Updates(map[string]any{
					"version": gorm.Expr("version + 1"),
					"utime":   now,
					"status":  domain.SendStatusFailed.String(),
				}).Error
		}
		return nil
	})
}

func (d *notificationDAO) batchMarkSuccess(tx *gorm.DB, successIds []uint64) error {
	now := time.Now().Unix()
	err := tx.Model(&Notification{}).
		Where("id IN ?", successIds).
		Updates(map[string]any{
			"version": gorm.Expr("version + 1"),
			"utime":   now,
			"status":  domain.SendStatusSucceeded.String(),
		}).Error
	if err != nil {
		return err
	}

	// 要更新 callback log 了
	return tx.Model(&CallbackLog{}).
		Where("notification_id IN ", successIds).
		Updates(map[string]any{
			"status": domain.CallbackLogStatusPending.String(),
			"utime":  now,
		}).Error
}

func (d *notificationDAO) FindReadyNotifications(ctx context.Context, offset, limit int) ([]Notification, error) {
	var res []Notification
	now := time.Now().UnixMilli()
	err := d.db.WithContext(ctx).
		Where("scheduled_stime <=? AND scheduled_etime >= ? AND status=?", now, now, string(domain.SendStatusPending)).
		Limit(limit).Offset(offset).
		Find(&res).Error
	return res, err
}

func (d *notificationDAO) MarkSuccess(ctx context.Context, notification Notification) error {
	now := time.Now().UnixMilli()
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&Notification{}).
			Where("id = ?", notification.ID).
			Updates(map[string]any{
				"status":  notification.Status,
				"utime":   now,
				"version": gorm.Expr("version + 1"),
			}).Error
		if err != nil {
			return err
		}
		// 要把 callback log 标记为可以发送了
		return tx.Model(&CallbackLog{}).Where("notification_id = ?").Updates(map[string]any{
			// 标记为可以发送回调了
			"status": domain.CallbackLogStatusPending,
			"utime":  now,
		}).Error
	})
}

func (d *notificationDAO) MarkFailed(ctx context.Context, notification Notification) error {
	now := time.Now().UnixMilli()
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&Notification{}).
			Where("id = ?", notification.ID).
			Updates(map[string]any{
				"status":  notification.Status,
				"utime":   now,
				"version": gorm.Expr("version + 1"),
			}).Error
		if err != nil {
			return err
		}
		return tx.Model(&Quota{}).Where("biz_id = ? AND channel = ?",
			notification.BizID, notification.Channel).
			Updates(map[string]any{
				"quota": gorm.Expr("quota+1"),
				"utime": now,
			}).Error
	})
}

func (d *notificationDAO) MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error) {
	now := time.Now()
	ddl := now.Add(-time.Minute).UnixMilli()
	sub := d.db.Model(&Notification{}).
		Select("id").
		Limit(batchSize).
		Where("status = ? AND utime <=?", string(domain.SendStatusSending), ddl)
	res := d.db.WithContext(ctx).Where("IN ?", sub).Updates(map[string]any{
		"status":  string(domain.SendStatusFailed),
		"version": gorm.Expr("version + 1"),
		"utime":   now,
	})
	return res.RowsAffected, res.Error
}
