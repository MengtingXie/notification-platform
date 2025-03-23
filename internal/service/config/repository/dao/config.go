package dao

import (
	"gitee.com/flycash/notification-platform/internal/pkg/dao"
)

// BusinessConfig 业务配置表
type BusinessConfig struct {
	ID            int64    `gorm:"primary_key"`
	BizID         string   `gorm:"primaryKey;type:VARCHAR(64);comment:'业务标识'"`
	ChannelConfig dao.JSON `gorm:"type:JSON;NOT NULL;comment:'{\"allowed_channels\":[\"SMS\",\"EMAIL\"], \"default\":\"SMS\"}'"`
	RateLimit     int      `gorm:"type:INT;DEFAULT:1000;comment:'每秒最大请求数'"`
	Quota         dao.JSON `gorm:"type:JSON;comment:'{\"monthly\":{\"SMS\":100000,\"EMAIL\":500000}}'"`
	RetryPolicy   dao.JSON `gorm:"type:JSON;comment:'{\"max_attempts\":3, \"backoff\":\"EXPONENTIAL\"}'"`
	Ctime         int64
	Utime         int64
}

// TableName 重命名表
func (BusinessConfig) TableName() string {
	return "business_config"
}
