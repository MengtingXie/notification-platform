package dao

import (
	"context"
	"errors"
	"time"

	"github.com/ego-component/egorm"
	"gorm.io/gorm"
)

// ChannelTemplate 渠道模板表
type ChannelTemplate struct {
	ID              int64  `gorm:"primaryKey;autoIncrement;comment:'渠道模版ID'"`
	OwnerID         int64  `gorm:"type:BIGINT;NOT NULL;comment:'用户ID或部门ID'"`
	OwnerType       string `gorm:"type:ENUM('person', 'organization');NOT NULL;comment:'业务方类型：person-个人,organization-组织'"`
	Name            string `gorm:"type:VARCHAR(128);NOT NULL;comment:'模板名称'"`
	Description     string `gorm:"type:VARCHAR(512);NOT NULL;comment:'模板描述'"`
	Channel         string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;comment:'渠道类型'"`
	BusinessType    int64  `gorm:"type:BIGINT;NOT NULL;DEFAULT:1;comment:'业务类型：1-推广营销、2-通知、3-验证码等'"`
	ActiveVersionID int64  `gorm:"type:BIGINT;DEFAULT:0;index:idx_active_version;comment:'当前启用的版本ID，0表示无活跃版本'"`
	Ctime           int64
	Utime           int64
}

// TableName 重命名表
func (ChannelTemplate) TableName() string {
	return "channel_templates"
}

// ChannelTemplateVersion 渠道模板版本表
type ChannelTemplateVersion struct {
	ID                int64  `gorm:"primaryKey;autoIncrement;comment:'渠道模版版本ID'"`
	ChannelTemplateID int64  `gorm:"type:BIGINT;NOT NULL;index:idx_channel_template_id;comment:'关联渠道模版ID'"`
	Name              string `gorm:"type:VARCHAR(32);NOT NULL;comment:'版本名称，如v1.0.0'"`
	Signature         string `gorm:"type:VARCHAR(64);comment:'已通过所有供应商审核的短信签名/邮件发件人'"`
	Content           string `gorm:"type:TEXT;NOT NULL;comment:'原始模板内容，使用平台统一变量格式，如${name}'"`
	Remark            string `gorm:"type:TEXT;NOT NULL;comment:'申请说明,描述使用短信的业务场景，并提供短信完整示例（填入变量内容），信息完整有助于提高模板审核通过率。'"`
	// 审核相关信息，AuditID之后的为冗余的信息
	AuditID                  int64  `gorm:"type:BIGINT;NOT NULL;DEFAULT:0;comment:'审核表ID, 0表示尚未提交审核或者未拿到审核结果'"`
	AuditorID                int64  `gorm:"type:BIGINT;comment:'审核人ID'"`
	AuditTime                int64  `gorm:"comment:'审核时间'"`
	AuditStatus              string `gorm:"type:ENUM('PENDING','IN_REVIEW','REJECTED','APPROVED');NOT NULL;DEFAULT:'PENDING';comment:'内部审核状态，PENDING表示未提交审核；IN_REVIEW表示已提交审核；APPROVED表示审核通过；REJECTED表示审核未通过'"`
	RejectReason             string `gorm:"type:VARCHAR(512);comment:'拒绝原因'"`
	LastReviewSubmissionTime int64  `gorm:"comment:'上一次提交审核时间'"`
	Ctime                    int64
	Utime                    int64
}

// TableName 重命名表
func (ChannelTemplateVersion) TableName() string {
	return "channel_template_versions"
}

// ChannelTemplateProvider 渠道模供应商关联表
type ChannelTemplateProvider struct {
	ID                       int64  `gorm:"primaryKey;autoIncrement;comment:'渠道模版-供应商关联ID'"`
	TemplateID               int64  `gorm:"type:BIGINT;NOT NULL;uniqueIndex:idx_template_version_provider,priority:1;uniqueIndex:idx_tmpl_ver_name_chan,priority:1;comment:'渠道模版ID'"`
	TemplateVersionID        int64  `gorm:"type:BIGINT;NOT NULL;uniqueIndex:idx_template_version_provider,priority:2;uniqueIndex:idx_tmpl_ver_name_chan,priority:2;comment:'渠道模版版本ID'"`
	ProviderID               int64  `gorm:"type:BIGINT;NOT NULL;uniqueIndex:idx_template_version_provider,priority:3;comment:'供应商ID'"`
	ProviderName             string `gorm:"type:VARCHAR(64);NOT NULL;uniqueIndex:idx_tmpl_ver_name_chan,priority:3;comment:'供应商名称'"`
	ProviderChannel          string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;uniqueIndex:idx_tmpl_ver_name_chan,priority:4;comment:'渠道类型'"`
	RequestID                string `gorm:"type:VARCHAR(256);comment:'审核请求在供应商侧的ID，用于排查问题'"`
	ProviderTemplateID       string `gorm:"type:VARCHAR(256);comment:'当前版本模版在供应商侧的ID，审核通过后才会有值'"`
	AuditStatus              string `gorm:"type:ENUM('PENDING','IN_REVIEW','REJECTED','APPROVED');NOT NULL;DEFAULT:'PENDING';comment:'供应商侧模版审核状态，PENDING表示未提交审核；IN_REVIEW表示已提交审核；APPROVED表示审核通过；REJECTED表示审核未通过'"`
	RejectReason             string `gorm:"type:VARCHAR(512);comment:'供应商侧拒绝原因'"`
	LastReviewSubmissionTime int64  `gorm:"comment:'上一次提交审核时间'"`
	Ctime                    int64
	Utime                    int64
}

// TableName 重命名表
func (ChannelTemplateProvider) TableName() string {
	return "channel_template_providers"
}

// ChannelTemplateDAO 模板数据访问接口
type ChannelTemplateDAO interface {
	// 模版

	// CreateTemplate 创建模板
	CreateTemplate(ctx context.Context, template ChannelTemplate) (ChannelTemplate, error)
	// GetTemplatesByOwner 根据所有者获取模板列表
	GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType string) ([]ChannelTemplate, error)
	// GetTemplateByID 根据ID获取模板
	GetTemplateByID(ctx context.Context, id int64) (ChannelTemplate, error)
	// UpdateTemplate 更新模板
	UpdateTemplate(ctx context.Context, template ChannelTemplate) error
	// SetActiveVersion 设置模板活跃版本
	SetActiveVersion(ctx context.Context, templateID, versionID int64) error

	// 模版版本

	// CreateTemplateVersion 创建模板版本
	CreateTemplateVersion(ctx context.Context, version ChannelTemplateVersion) (ChannelTemplateVersion, error)
	// GetVersionsByTemplateIDs 根据模板IDs获取版本列表
	GetVersionsByTemplateIDs(ctx context.Context, templateIDs []int64) ([]ChannelTemplateVersion, error)
	// GetTemplateVersionByID 根据ID获取模板版本
	GetTemplateVersionByID(ctx context.Context, versionID int64) (ChannelTemplateVersion, error)

	// 供应商关联

	// BatchCreateTemplateProviders 批量创建模板供应商关联
	BatchCreateTemplateProviders(ctx context.Context, providers []ChannelTemplateProvider) ([]ChannelTemplateProvider, error)
	// GetProvidersByVersionIDs 根据版本IDs获取供应商关联
	GetProvidersByVersionIDs(ctx context.Context, versionIDs []int64) ([]ChannelTemplateProvider, error)
	// GetApprovedProvidersByTemplateIDAndVersionID 根据模版ID和版本ID查找审核通过的供应商
	GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]ChannelTemplateProvider, error)
}

// channelTemplateDAO DAO层实现
type channelTemplateDAO struct {
	db *egorm.Component
}

// NewChannelTemplateDAO 创建模板DAO实例
func NewChannelTemplateDAO(db *egorm.Component) ChannelTemplateDAO {
	return &channelTemplateDAO{
		db: db,
	}
}

// GetTemplatesByOwner 根据所有者获取模板列表
func (d *channelTemplateDAO) GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType string) ([]ChannelTemplate, error) {
	var templates []ChannelTemplate
	result := d.db.WithContext(ctx).Where("owner_id = ? AND owner_type = ?", ownerID, ownerType).Find(&templates)
	if result.Error != nil {
		return nil, result.Error
	}
	return templates, nil
}

// GetVersionsByTemplateIDs 根据模板IDs获取版本列表
func (d *channelTemplateDAO) GetVersionsByTemplateIDs(ctx context.Context, templateIDs []int64) ([]ChannelTemplateVersion, error) {
	if len(templateIDs) == 0 {
		return []ChannelTemplateVersion{}, nil
	}

	var versions []ChannelTemplateVersion
	result := d.db.WithContext(ctx).Where("channel_template_id IN ?", templateIDs).Find(&versions)
	if result.Error != nil {
		return nil, result.Error
	}
	return versions, nil
}

// GetProvidersByVersionIDs 根据版本IDs获取供应商关联
func (d *channelTemplateDAO) GetProvidersByVersionIDs(ctx context.Context, versionIDs []int64) ([]ChannelTemplateProvider, error) {
	if len(versionIDs) == 0 {
		return []ChannelTemplateProvider{}, nil
	}

	var providers []ChannelTemplateProvider
	result := d.db.WithContext(ctx).Where("template_version_id IN ?", versionIDs).Find(&providers)
	if result.Error != nil {
		return nil, result.Error
	}
	return providers, nil
}

// CreateTemplate 创建模板
func (d *channelTemplateDAO) CreateTemplate(ctx context.Context, template ChannelTemplate) (ChannelTemplate, error) {
	now := time.Now().Unix()
	template.Ctime = now
	template.Utime = now

	result := d.db.WithContext(ctx).Create(&template)
	if result.Error != nil {
		return ChannelTemplate{}, result.Error
	}
	return template, nil
}

// GetTemplateByID 根据ID获取模板
func (d *channelTemplateDAO) GetTemplateByID(ctx context.Context, id int64) (ChannelTemplate, error) {
	var template ChannelTemplate
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChannelTemplate{}, nil
		}
		return ChannelTemplate{}, err
	}
	return template, nil
}

// UpdateTemplate 更新模板
func (d *channelTemplateDAO) UpdateTemplate(ctx context.Context, template ChannelTemplate) error {
	// 只允许更新name、description、business_type这三个字段
	updateData := map[string]interface{}{
		"name":          template.Name,
		"description":   template.Description,
		"business_type": template.BusinessType,
		"utime":         time.Now().Unix(),
	}

	return d.db.WithContext(ctx).Model(&ChannelTemplate{}).
		Where("id = ?", template.ID).
		Updates(updateData).Error
}

// SetActiveVersion 设置模板活跃版本
func (d *channelTemplateDAO) SetActiveVersion(ctx context.Context, templateID, versionID int64) error {
	return d.db.WithContext(ctx).Model(&ChannelTemplate{}).
		Where("id = ?", templateID).
		Updates(map[string]interface{}{
			"active_version_id": versionID,
			"utime":             time.Now().Unix(),
		}).Error
}

// CreateTemplateVersion 创建模板版本
func (d *channelTemplateDAO) CreateTemplateVersion(ctx context.Context, version ChannelTemplateVersion) (ChannelTemplateVersion, error) {
	now := time.Now().Unix()
	version.Ctime = now
	version.Utime = now

	result := d.db.WithContext(ctx).Create(&version)
	if result.Error != nil {
		return ChannelTemplateVersion{}, result.Error
	}
	return version, nil
}

// GetTemplateVersionByID 根据ID获取模板版本
func (d *channelTemplateDAO) GetTemplateVersionByID(ctx context.Context, versionID int64) (ChannelTemplateVersion, error) {
	var version ChannelTemplateVersion
	err := d.db.WithContext(ctx).Where("id = ?", versionID).First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChannelTemplateVersion{}, nil
		}
		return ChannelTemplateVersion{}, err
	}
	return version, nil
}

// BatchCreateTemplateProviders 批量创建模板供应商关联
func (d *channelTemplateDAO) BatchCreateTemplateProviders(ctx context.Context, providers []ChannelTemplateProvider) ([]ChannelTemplateProvider, error) {
	if len(providers) == 0 {
		return []ChannelTemplateProvider{}, nil
	}

	now := time.Now().Unix()
	for i := range providers {
		providers[i].Ctime = now
		providers[i].Utime = now
	}

	result := d.db.WithContext(ctx).Create(&providers)
	if result.Error != nil {
		return nil, result.Error
	}
	return providers, nil
}

// GetApprovedProvidersByTemplateIDAndVersionID 根据模版ID和版本ID查找审核通过的供应商
func (d *channelTemplateDAO) GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]ChannelTemplateProvider, error) {
	var providers []ChannelTemplateProvider
	err := d.db.WithContext(ctx).Model(&ChannelTemplateProvider{}).
		Where("template_id = ? AND template_version_id = ? AND audit_status = ?",
			templateID, versionID, "APPROVED").Find(&providers).Error
	return providers, err
}
