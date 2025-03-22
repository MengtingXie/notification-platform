package dao

import (
	"context"
	"time"
)

type NotificationDAO interface {
	Create(ctx context.Context, data *Notification) error
}

// Notification 通知记录表
type Notification struct {
	ID             int64      `gorm:"primaryKey;comment:'雪花算法ID'"`
	BizID          string     `gorm:"type:VARCHAR(64);NOT NULL;index:idx_biz_status,priority:1;comment:'业务方唯一标识'"`
	Receiver       string     `gorm:"type:VARCHAR(256);NOT NULL;comment:'接收者(手机/邮箱/用户ID)'"`
	Channel        string     `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;comment:'发送渠道'"`
	TemplateID     int64      `gorm:"type:BIGINT;NOT NULL;comment:'关联模板ID'"`
	Content        string     `gorm:"type:TEXT;NOT NULL;comment:'渲染后的内容(加密存储)'"`
	Status         string     `gorm:"type:ENUM('PENDING','SENT','FAILED');DEFAULT:'PENDING';index:idx_biz_status,priority:2;comment:'发送状态'"`
	RetryCount     int8       `gorm:"type:TINYINT;DEFAULT:0;comment:'当前重试次数'"`
	ScheduledSTime *time.Time `gorm:"type:DATETIME;index:idx_scheduled;comment:'计划发送开始时间'"`
	ScheduledETime *time.Time `gorm:"type:DATETIME;comment:'计划发送结束时间'"`
	CreatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP;comment:'创建时间'"`
	UpdatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
}

// TableName 重命名表
func (Notification) TableName() string {
	return "notification"
}

// TransactionNotification 事务通知表
type TransactionNotification struct {
	TxID           string     `gorm:"primaryKey;type:VARCHAR(128);comment:'事务ID'"`
	NotificationID int64      `gorm:"type:BIGINT;NOT NULL;comment:'关联通知ID'"`
	Status         string     `gorm:"type:ENUM('PREPARED','COMMITTED','CANCELED');NOT NULL;index:idx_status_check,priority:1;comment:'事务状态'"`
	CheckCount     int8       `gorm:"type:TINYINT;DEFAULT:0;index:idx_status_check,priority:2;comment:'回查次数'"`
	LastCheckTime  *time.Time `gorm:"type:DATETIME;comment:'最后回查时间'"`
	PreparedData   string     `gorm:"type:TEXT;comment:'预提交数据'"`
	CreatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP;comment:'创建时间'"`
	UpdatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
}

// TableName 重命名表
func (TransactionNotification) TableName() string {
	return "transaction_notification"
}
