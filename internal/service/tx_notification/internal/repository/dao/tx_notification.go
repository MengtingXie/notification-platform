package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/domain"

	"github.com/ego-component/egorm"
)

var (
	ErrDuplicatedTx       = errors.New("duplicated tx")
	ErrUpdateStatusFailed = errors.New("没有更新")
)

type TxNotification struct {
	// 事务id
	TxID int64  `gorm:"column:tx_id;autoIncrement;primaryKey"`
	Key  string `gorm:"type:VARCHAR(256);NOT NULL;uniqueIndex:idx_biz_id_key,priority:2;comment:'业务内唯一标识，区分同一个业务内的不同通知'"`
	// 创建的通知id
	NotificationID uint64 `gorm:"column:notification_id"`
	// 业务方唯一标识
	BizID int64 `gorm:"column:biz_id;type:bigint;not null;uniqueIndex:idx_biz_id_key"`
	// 通知状态
	Status string `gorm:"column:status;type:varchar(20);not null;default:'PREPARE';index:idx_next_check_time_status"`
	// 检查次数
	CheckCount int `gorm:"column:check_count;type:int;not null;default:0"`
	// 下一次的回查时间戳
	NextCheckTime int64 `gorm:"column:next_check_time;type:bigint;not null;default:0;index:idx_next_check_time_status"`
	// 创建时间
	Ctime int64 `gorm:"column:ctime;type:bigint;not null"`
	// 更新时间
	Utime int64 `gorm:"column:utime;type:bigint;not null"`
}

// TableName specifies the table name for the TxNotification model
func (t *TxNotification) TableName() string {
	return "tx_notifications"
}

type TxNotificationDAO interface {
	// Create 直接保存即可
	Create(ctx context.Context, notification TxNotification) (int64, error)
	// Find 查找需要回查的事务通知，筛选条件是status为PREPARE，并且下一次回查时间小于当前时间
	Find(ctx context.Context, offset, limit int) ([]TxNotification, error)
	// UpdateStatus 变更状态 用于用户提交/取消
	UpdateStatus(ctx context.Context, txID int64, status string) error
	// UpdateCheckStatus 更新回查状态用于回查任务，回查次数+1 更新下一次的回查时间戳，通知状态，utime
	UpdateCheckStatus(ctx context.Context, txNotifications []TxNotification) error
	// First 通过事务id查找对应的事务
	First(ctx context.Context, txID int64) (TxNotification, error)
	// BatchGetTxNotification 批量获取事务消息
	BatchGetTxNotification(ctx context.Context, txIDs []int64) (map[int64]TxNotification, error)

	GetByBizIDKey(ctx context.Context, bizID int64, key string) (TxNotification, error)
	UpdateNotificationID(ctx context.Context, bizID int64, key string, notificationID uint64) error
}

type txNotificationDAO struct {
	db *egorm.Component
}

func (t *txNotificationDAO) UpdateNotificationID(ctx context.Context, bizID int64, key string, notificationID uint64) error {
	err := t.db.WithContext(ctx).
		Model(&TxNotification{}).
		Where("biz_id = ? AND `key` = ?", bizID, key).
		Update("notification_id", notificationID).Error
	return err
}

// NewTxNotificationDAO creates a new instance of TxNotificationDAO
func NewTxNotificationDAO(db *egorm.Component) TxNotificationDAO {
	return &txNotificationDAO{
		db: db,
	}
}

func (t *txNotificationDAO) GetByBizIDKey(ctx context.Context, bizID int64, key string) (TxNotification, error) {
	var tx TxNotification
	err := t.db.WithContext(ctx).
		Model(&TxNotification{}).
		Where("biz_id = ? AND `key` = ?", bizID, key).First(&tx).Error

	return tx, err
}

func (t *txNotificationDAO) BatchGetTxNotification(ctx context.Context, txIDs []int64) (map[int64]TxNotification, error) {
	var txns []TxNotification
	err := t.db.WithContext(ctx).Where("tx_id in (?)", txIDs).Find(&txns).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64]TxNotification, len(txns))
	for id := range txns {
		txn := txns[id]
		result[txn.TxID] = txn
	}
	return result, nil
}

func (t *txNotificationDAO) First(ctx context.Context, txID int64) (TxNotification, error) {
	var notification TxNotification
	err := t.db.WithContext(ctx).Where("tx_id = ?", txID).First(&notification).Error
	return notification, err
}

func (t *txNotificationDAO) Create(ctx context.Context, notification TxNotification) (int64, error) {
	// Set create and update time if not already set
	now := time.Now().UnixMilli()
	notification.Ctime = now
	notification.Utime = now
	err := t.db.WithContext(ctx).Create(&notification).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return 0, ErrDuplicatedTx
	}
	return notification.TxID, err
}

func (t *txNotificationDAO) Find(ctx context.Context, offset, limit int) ([]TxNotification, error) {
	var notifications []TxNotification
	currentTime := time.Now().UnixMilli()

	err := t.db.WithContext(ctx).
		Where("status = ? AND next_check_time <= ? AND next_check_time > 0", domain.TxNotificationStatusPrepare, currentTime).
		Offset(offset).
		Limit(limit).
		Order("next_check_time").
		Find(&notifications).Error

	return notifications, err
}

func (t *txNotificationDAO) UpdateStatus(ctx context.Context, txID int64, status string) error {
	now := time.Now().UnixMilli()
	// 只能更新Prepare状态的
	res := t.db.WithContext(ctx).
		Model(&TxNotification{}).
		Where("tx_id = ? AND status = ?", txID, domain.TxNotificationStatusPrepare.String()).
		Updates(map[string]any{
			"status": status,
			"utime":  now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUpdateStatusFailed
	}
	return nil
}

// 只更新
func (t *txNotificationDAO) UpdateCheckStatus(ctx context.Context, txNotifications []TxNotification) error {
	sqls := make([]string, 0, len(txNotifications))
	now := time.Now().UnixMilli()
	for _, txNotification := range txNotifications {
		updateSQL := fmt.Sprintf("UPDATE `tx_notifications` set `status` = '%s',`utime` = %d ,`next_check_time` = %d,`check_count` = %d WHERE `key` = %s AND `biz_id` = %d AND `status` = 'PREPARE'", txNotification.Status, now, txNotification.NextCheckTime, txNotification.CheckCount, txNotification.Key, txNotification.BizID)
		sqls = append(sqls, updateSQL)
	}
	// 拼接所有SQL并执行
	if len(sqls) > 0 {
		combinedSQL := strings.Join(sqls, "; ")
		return t.db.WithContext(ctx).Exec(combinedSQL).Error
	}
	return nil
}
