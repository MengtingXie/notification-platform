package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ego-component/egorm"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrNotificationDuplicate       = errors.New("通知记录主键冲突")
	ErrNotificationNotFound        = errors.New("通知记录不存在")
	ErrNotificationVersionMismatch = errors.New("通知记录版本不匹配")
)

const (
	notificationStatusPending   = "PENDING"
	notificationStatusSucceeded = "SUCCEEDED"
	notificationStatusFailed    = "FAILED"
)

type NotificationDAO interface {
	// Create 创建单条通知记录
	Create(ctx context.Context, data Notification) (Notification, error)

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, status string, version int) error

	// BatchCreate 批量创建通知记录
	BatchCreate(ctx context.Context, dataList []Notification) ([]Notification, error)

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败，使用乐观锁控制并发
	// successNotifications: 更新为成功状态的通知列表，包含ID、Version和重试次数
	// failedNotifications: 更新为失败状态的通知列表，包含ID、Version和重试次数
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, successNotifications, failedNotifications []Notification) error

	// GetByID 根据ID查询通知
	GetByID(ctx context.Context, id uint64) (Notification, error)

	BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]Notification, error)

	// GetByBizID 根据业务ID查询通知列表
	GetByBizID(ctx context.Context, bizID int64) ([]Notification, error)

	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]Notification, error)

	// ListByStatus 根据状态查询通知列表
	ListByStatus(ctx context.Context, status string, limit int) ([]Notification, error)

	// ListByScheduleTime 根据计划发送时间查询通知
	ListByScheduleTime(ctx context.Context, startTime, endTime int64, limit int) ([]Notification, error)

	// BatchUpdateStatus 批量更新通知状态
	BatchUpdateStatus(ctx context.Context, ids []uint64, status string) error
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
	Status            string `gorm:"type:ENUM('PREPARE','CANCELED','PENDING','SUCCEEDED','FAILED');DEFAULT:'PENDING';index:idx_biz_id_status,priority:2;comment:'发送状态'"`
	RetryCount        int8   `gorm:"type:TINYINT;DEFAULT:0;comment:'当前重试次数'"`
	ScheduledSTime    int64  `gorm:"index:idx_scheduled,priority:1;comment:'计划发送开始时间'"`
	ScheduledETime    int64  `gorm:"index:idx_scheduled,priority:2;comment:'计划发送结束时间'"`
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

// Create 创建单条通知记录
func (d *notificationDAO) Create(ctx context.Context, data Notification) (Notification, error) {
	now := time.Now().Unix()
	data.Ctime, data.Utime = now, now
	data.Version = 1

	err := d.db.WithContext(ctx).Create(&data).Error
	if isUniqueConstraintError(err) {
		return Notification{}, fmt.Errorf("%w", ErrNotificationDuplicate)
	}
	return data, err
}

// isUniqueConstraintError 检查是否是唯一索引冲突错误
func isUniqueConstraintError(err error) bool {
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

// BatchCreate 批量创建通知记录
func (d *notificationDAO) BatchCreate(ctx context.Context, datas []Notification) ([]Notification, error) {
	if len(datas) == 0 {
		return nil, nil
	}

	now := time.Now().Unix()
	for i := range datas {
		datas[i].Ctime, datas[i].Utime = now, now
		datas[i].Version = 1
	}

	// 使用事务执行批量插入，确保可以捕获并处理唯一键冲突
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range datas {
			err := tx.Create(&datas[i]).Error
			if isUniqueConstraintError(err) {
				return fmt.Errorf("%w", ErrNotificationDuplicate)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return datas, err
}

// UpdateStatus 更新通知状态
func (d *notificationDAO) UpdateStatus(ctx context.Context, id uint64, status string, version int) error {
	result := d.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]interface{}{
			"status":  status,
			"version": gorm.Expr("version + 1"),
			"utime":   time.Now().Unix(),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected < 1 {
		// 没有更新任何行，可能是ID不存在或版本不匹配
		// 先查询记录是否存在
		var count int64
		if err := d.db.WithContext(ctx).Model(&Notification{}).
			Where("id = ?", id).
			Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			return fmt.Errorf("%w", ErrNotificationNotFound)
		}

		return fmt.Errorf("%w", ErrNotificationVersionMismatch)
	}

	return nil
}

// GetByID 根据ID查询通知
func (d *notificationDAO) GetByID(ctx context.Context, id uint64) (Notification, error) {
	var notification Notification
	err := d.db.WithContext(ctx).First(&notification, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Notification{}, fmt.Errorf("%w: id=%d", ErrNotificationNotFound, id)
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

// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
func (d *notificationDAO) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]Notification, error) {
	var notifications []Notification
	err := d.db.WithContext(ctx).Where("biz_id = ? AND `key` IN ?", bizID, keys).Find(&notifications).Error
	if err != nil {
		return nil, fmt.Errorf("查询通知列表失败: %w", err)
	}
	return notifications, nil
}

// ListByStatus 根据状态查询通知列表
func (d *notificationDAO) ListByStatus(ctx context.Context, status string, limit int) ([]Notification, error) {
	var notifications []Notification
	query := d.db.WithContext(ctx).Where("status = ?", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

// ListByScheduleTime 根据计划发送时间查询通知
func (d *notificationDAO) ListByScheduleTime(ctx context.Context, startTime, endTime int64, limit int) ([]Notification, error) {
	var notifications []Notification
	query := d.db.WithContext(ctx).
		Where("scheduled_s_time >= ? AND scheduled_s_time <= ?", startTime, endTime).
		Order("scheduled_s_time ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败，使用乐观锁控制并发
// successNotifications: 更新为成功状态的通知列表，包含ID、Version和重试次数
// failedNotifications: 更新为失败状态的通知列表，包含ID、Version和重试次数
func (d *notificationDAO) BatchUpdateStatusSucceededOrFailed(ctx context.Context, successNotifications, failedNotifications []Notification) error {
	if len(successNotifications) == 0 && len(failedNotifications) == 0 {
		return nil
	}

	// 开启事务
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		var updateErrors []error

		// 处理成功状态的通知
		for i := range successNotifications {
			if err := d.updateNotificationStatus(tx, successNotifications[i], notificationStatusSucceeded, now); err != nil {
				updateErrors = append(updateErrors, err)
			}
		}

		// 处理失败状态的通知
		for i := range failedNotifications {
			if err := d.updateNotificationStatus(tx, failedNotifications[i], notificationStatusFailed, now); err != nil {
				updateErrors = append(updateErrors, err)
			}
		}

		// 如果有任何更新错误，则回滚事务
		if len(updateErrors) > 0 {
			return fmt.Errorf("批量更新通知状态失败: %v", updateErrors)
		}

		return nil
	})
}

// updateNotificationStatus 更新单个通知的状态，处理乐观锁和重试次数
func (d *notificationDAO) updateNotificationStatus(tx *gorm.DB, notification Notification, status string, now int64) error {
	// 设置基本更新字段
	updates := map[string]interface{}{
		"status":  status,
		"utime":   now,
		"version": gorm.Expr("version + 1"),
	}

	// 如果设置了重试次数，也需要更新
	if notification.RetryCount > 0 {
		updates["retry_count"] = notification.RetryCount
	}

	result := tx.Model(&Notification{}).
		Where("id = ? AND version = ?", notification.ID, notification.Version).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		// 版本不匹配或记录不存在
		var exists int64
		if err := tx.Model(&Notification{}).Where("id = ?", notification.ID).Count(&exists).Error; err != nil {
			return err
		}

		if exists == 0 {
			return fmt.Errorf("未找到ID为%d的通知记录", notification.ID)
		}

		return fmt.Errorf("ID为%d的通知记录版本不匹配", notification.ID)
	}

	return nil
}

func (d *notificationDAO) BatchUpdateStatus(ctx context.Context, ids []uint64, status string) error {
	result := d.db.WithContext(ctx).Model(&Notification{}).
		Where("id in ?", ids).
		Updates(map[string]interface{}{
			"status":  status,
			"utime":   time.Now().Unix(),
			"version": gorm.Expr("version + 1"),
		})
	return result.Error
}
