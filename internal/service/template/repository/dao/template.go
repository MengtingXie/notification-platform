package dao

import (
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/dao"
)

// ChannelTemplate 渠道模板表
type ChannelTemplate struct {
	ID           int64     `gorm:"primaryKey"`
	ProviderID   int64     `gorm:"type:BIGINT;NOT NULL;index:idx_provider;comment:'关联供应商ID'"`
	TemplateCode string    `gorm:"type:VARCHAR(128);NOT NULL;comment:'第三方模板ID'"`
	Content      string    `gorm:"type:TEXT;NOT NULL;comment:'原始模板内容'"`
	Variables    dao.JSON  `gorm:"type:JSON;comment:'{\"code\":\"required\",\"name\":\"optional\"}'"` // 使用自定义JSON类型
	AuditStatus  string    `gorm:"type:ENUM('PENDING','APPROVED');DEFAULT:'PENDING';comment:'审核状态'"`
	Signature    string    `gorm:"type:VARCHAR(64);comment:'短信签名/邮件发件人'"`
	CreatedAt    time.Time `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP;comment:'创建时间'"`
	UpdatedAt    time.Time `gorm:"type:DATETIME;DEFAULT:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
	// Provider     ServiceProvider `gorm:"foreignKey:ProviderID"` // 外键关联
}

// TableName 重命名表
func (ChannelTemplate) TableName() string {
	return "channel_template"
}

// ServiceProvider 服务商表（假设存在）
type ServiceProvider struct {
	ID int64 `gorm:"primaryKey"`
	// 其他字段...
}
