package dao

import "time"

// CallbackLog 回调记录表
type CallbackLog struct {
	CallbackID     int64      `gorm:"primaryKey"`
	NotificationID int64      `gorm:"type:BIGINT;NOT NULL;index:idx_notification_status,priority:1;comment:'通知ID'"`
	CallbackURL    string     `gorm:"type:VARCHAR(512);NOT NULL;comment:'回调URL'"`
	RequestBody    string     `gorm:"type:TEXT;comment:'回调内容'"`
	ResponseCode   int16      `gorm:"type:SMALLINT;index:idx_notification_status,priority:2;comment:'HTTP状态码'"`
	RetryCount     int8       `gorm:"type:TINYINT;DEFAULT:0;comment:'重试次数'"`
	NextRetryTime  *time.Time `gorm:"type:DATETIME;comment:'下次重试时间'"`
	CreatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP;comment:'创建时间'"`
	UpdatedAt      time.Time  `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
	// Notification   Notification `gorm:"foreignKey:NotificationID"` // 外键关联
}

// TableName 重命名表
func (CallbackLog) TableName() string {
	return "callback_log"
}
