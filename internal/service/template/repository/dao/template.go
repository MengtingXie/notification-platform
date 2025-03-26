package dao

import (
	"gitee.com/flycash/notification-platform/internal/pkg/dao"
)

// ChannelTemplate 渠道模板表
type ChannelTemplate struct {
	ID          int64    `gorm:"primaryKey;"`
	ProviderID  int64    `gorm:"type:BIGINT;NOT NULL;index:idx_provider;comment:'关联供应商ID'"`
	Content     string   `gorm:"type:TEXT;NOT NULL;comment:'原始模板内容'"`
	Variables   dao.JSON `gorm:"type:JSON;comment:'{\"code\":\"required\",\"name\":\"optional\"}'"`
	AuditStatus string   `gorm:"type:ENUM('PENDING','REJECTED','APPROVED');DEFAULT:'PENDING';comment:'审核状态'"`
	Signature   string   `gorm:"type:VARCHAR(64);comment:'短信签名/邮件发件人'"`
	Ctime       int64
	Utime       int64
}

// TableName 重命名表
func (ChannelTemplate) TableName() string {
	return "channel_templates"
}

// Provider
// 1, name, api, price, 价格，channel(SMS,EMAIL,WECHAT),
// 1, name, api,
