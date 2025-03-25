package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrNotificationDuplicate = errors.New("通知记录主键冲突")

type NotificationDAO interface {
	// Create 创建单条通知记录
	Create(ctx context.Context, data Notification) error

	// UpdateStatus 更新通知状态
	UpdateStatus(ctx context.Context, id uint64, bizID string, status NotificationStatus) error

	// BatchCreate 批量创建通知记录
	BatchCreate(ctx context.Context, dataList []Notification) error

	// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
	// successIDs: 更新为成功状态的ID列表
	// failedNotifications: 更新为失败状态的通知列表，包含ID和重试次数
	BatchUpdateStatusSucceededOrFailed(ctx context.Context, successIDs []uint64, failedNotifications []Notification) error

	// FindByID 根据ID查询通知
	FindByID(ctx context.Context, id uint64) (Notification, error)

	// FindByBizID 根据业务ID查询通知列表
	FindByBizID(ctx context.Context, bizID string) ([]Notification, error)

	// ListByStatus 根据状态查询通知列表
	ListByStatus(ctx context.Context, status NotificationStatus, limit int) ([]Notification, error)

	// ListByScheduleTime 根据计划发送时间查询通知
	ListByScheduleTime(ctx context.Context, startTime, endTime int64, limit int) ([]Notification, error)
}

// Notification 通知记录表
type Notification struct {
	// Key   string `gorm:"primary_key;comment:'业务方内唯一'"`
	ID    uint64 `gorm:"primaryKey;comment:'雪花算法ID'"`
	BizID string `gorm:"type:VARCHAR(64);NOT NULL;index:idx_biz_status,priority:1;comment:'业务方唯一标识'"`
	// TaskID         string              `gorm:"type:VARCHAR(64);index:idx_task_id;comment:'批量任务ID'"`
	Receiver       string              `gorm:"type:VARCHAR(256);NOT NULL;comment:'接收者(手机/邮箱/用户ID)'"`
	Channel        NotificationChannel `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;comment:'发送渠道'"`
	TemplateID     int64               `gorm:"type:BIGINT;NOT NULL;comment:'关联模板ID'"`
	Content        string              `gorm:"type:TEXT;NOT NULL;comment:'渲染后的内容(加密存储)'"`
	Status         NotificationStatus  `gorm:"type:ENUM('PREPARE','CANCELED','PENDING','SUCCEEDED','FAILED');DEFAULT:'PENDING';index:idx_biz_status,priority:2;comment:'发送状态'"`
	RetryCount     int8                `gorm:"type:TINYINT;DEFAULT:0;comment:'当前重试次数'"`
	ScheduledSTime int64               `gorm:"index:idx_scheduled;comment:'计划发送开始时间'"`
	ScheduledETime int64               `gorm:"comment:'计划发送结束时间'"`
	Ctime          int64
	Utime          int64
}

type notificationDAO struct {
	db *gorm.DB
}

// NewNotificationDAO 创建通知DAO实例
func NewNotificationDAO(db *gorm.DB) NotificationDAO {
	return &notificationDAO{
		db: db,
	}
}

// Create 创建单条通知记录
func (n *notificationDAO) Create(ctx context.Context, data Notification) error {
	now := time.Now().Unix()
	data.Ctime, data.Utime = now, now
	err := n.db.WithContext(ctx).Create(&data).Error
	me := new(mysql.MySQLError)
	if ok := errors.As(err, &me); ok {
		const uniqueIndexErrNo uint16 = 1062
		if me.Number == uniqueIndexErrNo {
			return ErrNotificationDuplicate
		}
	}
	return err
}

// BatchCreate 批量创建通知记录
func (n *notificationDAO) BatchCreate(ctx context.Context, dataList []Notification) error {
	if len(dataList) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for i := range dataList {
		dataList[i].Ctime, dataList[i].Utime = now, now
	}
	return n.db.WithContext(ctx).CreateInBatches(dataList, len(dataList)).Error
}

// UpdateStatus 更新通知状态
func (n *notificationDAO) UpdateStatus(ctx context.Context, id uint64, bizID string, status NotificationStatus) error {
	result := n.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND biz_id = ?", id, bizID).
		Updates(map[string]interface{}{
			"status": status,
			"utime":  time.Now().Unix(),
		})
	return result.Error
}

// FindByID 根据ID查询通知
func (n *notificationDAO) FindByID(ctx context.Context, id uint64) (Notification, error) {
	var notification Notification
	err := n.db.WithContext(ctx).First(&notification, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Notification{}, fmt.Errorf("通知记录不存在: id=%d", id)
		}
		return Notification{}, err
	}
	return notification, nil
}

// FindByBizID 根据业务ID查询通知列表
func (n *notificationDAO) FindByBizID(ctx context.Context, bizID string) ([]Notification, error) {
	var notifications []Notification
	err := n.db.WithContext(ctx).Where("biz_id = ?", bizID).Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

// ListByStatus 根据状态查询通知列表
func (n *notificationDAO) ListByStatus(ctx context.Context, status NotificationStatus, limit int) ([]Notification, error) {
	var notifications []Notification
	query := n.db.WithContext(ctx).Where("status = ?", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

// ListByScheduleTime 根据计划发送时间查询通知
func (n *notificationDAO) ListByScheduleTime(ctx context.Context, startTime, endTime int64, limit int) ([]Notification, error) {
	var notifications []Notification
	query := n.db.WithContext(ctx).
		Where("scheduled_s_time >= ? AND scheduled_s_time <= ?", startTime, endTime).
		Order("scheduled_s_time ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

// BatchUpdateStatusSucceededOrFailed 批量更新通知状态为成功或失败
// 使用多语句批处理方式: 成功的通知合并为一条语句，失败的每条单独一条语句
func (n *notificationDAO) BatchUpdateStatusSucceededOrFailed(ctx context.Context, successIDs []uint64, failedNotifications []Notification) error {
	if len(successIDs) == 0 && len(failedNotifications) == 0 {
		return nil
	}

	// 开启事务
	return n.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()

		// 构建SQL语句
		var sqls []string

		// 处理成功状态的通知 - 合并为一条SQL
		if len(successIDs) > 0 {
			idStrs := make([]string, len(successIDs))
			for i := range successIDs {
				idStrs[i] = fmt.Sprintf("%d", successIDs[i])
			}

			// 直接构建完整SQL，不使用参数绑定
			successSQL := fmt.Sprintf(
				"UPDATE `notifications` SET `status` = '%s', `utime` = %d WHERE `id` IN (%s)",
				NotificationStatusSucceeded,
				now,
				strings.Join(idStrs, ", "),
			)
			sqls = append(sqls, successSQL)
		}

		// 处理失败状态的通知 - 每条单独一条SQL
		for i := range failedNotifications {
			var failedSQL string
			n := failedNotifications[i]
			if n.RetryCount > 0 {
				// 更新状态和重试次数
				failedSQL = fmt.Sprintf(
					"UPDATE `notifications` SET `status` = '%s', `retry_count` = %d, `utime` = %d WHERE `id` = %d",
					NotificationStatusFailed,
					n.RetryCount,
					now,
					n.ID,
				)
			} else {
				// 只更新状态
				failedSQL = fmt.Sprintf(
					"UPDATE `notifications` SET `status` = '%s', `utime` = %d WHERE `id` = %d",
					NotificationStatusFailed,
					now,
					n.ID,
				)
			}

			sqls = append(sqls, failedSQL)
		}

		// 拼接所有SQL并执行
		if len(sqls) > 0 {
			combinedSQL := strings.Join(sqls, "; ")

			err := tx.Exec(combinedSQL).Error
			if err != nil {
				fmt.Println(combinedSQL)
			}
			return err
		}

		return nil
	})
}
