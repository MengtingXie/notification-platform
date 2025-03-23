package dao

import (
	"context"
)

type NotificationDAO interface {
	Create(ctx context.Context, data *Notification) error
}

// Notification 通知记录表
type Notification struct {
	ID             int64  `gorm:"primaryKey;comment:'雪花算法ID'"`
	BizID          string `gorm:"type:VARCHAR(64);NOT NULL;index:idx_biz_status,priority:1;comment:'业务方唯一标识'"`
	Receiver       string `gorm:"type:VARCHAR(256);NOT NULL;comment:'接收者(手机/邮箱/用户ID)'"`
	Channel        string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;comment:'发送渠道'"`
	TemplateID     int64  `gorm:"type:BIGINT;NOT NULL;comment:'关联模板ID'"`
	Content        string `gorm:"type:TEXT;NOT NULL;comment:'渲染后的内容(加密存储)'"`
	Status         string `gorm:"type:ENUM('PREPARE','CANCEL','PENDING','SENT','FAILED');DEFAULT:'PENDING';index:idx_biz_status,priority:2;comment:'发送状态'"`
	RetryCount     int8   `gorm:"type:TINYINT;DEFAULT:0;comment:'当前重试次数'"`
	ScheduledSTime int64
	ScheduledETime int64
	Ctime          int64
	Utime          int64
}

// TableName 重命名表
func (Notification) TableName() string {
	return "notification"
}
