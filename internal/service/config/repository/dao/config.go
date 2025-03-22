package dao

import (
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/dao"
)

// BusinessConfig 业务方配置表
type BusinessConfig struct {
	BizID         string    `gorm:"primaryKey;type:VARCHAR(64);comment:'业务方标识'"`
	ChannelConfig dao.JSON  `gorm:"type:JSON;NOT NULL;comment:'{\"allowed_channels\":[\"SMS\",\"EMAIL\"], \"default\":\"SMS\"}'"`
	RateLimit     int       `gorm:"type:INT;DEFAULT:1000;comment:'每秒最大请求数'"`
	Quota         dao.JSON  `gorm:"type:JSON;comment:'{\"monthly\":{\"SMS\":100000,\"EMAIL\":500000}}'"`
	RetryPolicy   dao.JSON  `gorm:"type:JSON;comment:'{\"max_attempts\":3, \"backoff\":\"EXPONENTIAL\"}'"`
	CreatedAt     time.Time `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP;comment:'创建时间'"`
	UpdatedAt     time.Time `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
}

// TableName 重命名表
func (BusinessConfig) TableName() string {
	return "business_config"
}
