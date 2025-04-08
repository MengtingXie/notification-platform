package repository

import (
	"context"
	"errors"

	"gitee.com/flycash/notification-platform/internal/service/template/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/repository/dao"
)

// 添加错误定义
var (
	ErrTemplateNotFound           = errors.New("模板未找到")
	ErrTemplateVersionNotFound    = errors.New("模板版本未找到")
	ErrTemplateVersionNotApproved = errors.New("模板版本未审核通过")
	ErrInvalidParameter           = errors.New("参数无效")
)

// ChannelTemplateRepository 模板仓储接口
type ChannelTemplateRepository interface {
	// 模版

	// GetTemplates 根据所有者获取模板列表
	GetTemplates(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error)
	// CreateTemplate 创建模板
	CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error)
	// UpdateTemplate 更新模版
	UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error
	// SetActiveVersion 设置激活版本
	SetActiveVersion(ctx context.Context, templateID, versionID int64) error

	// 模版版本

	// CreateTemplateVersion 创建模板版本
	CreateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) (domain.ChannelTemplateVersion, error)
	// GetTemplateVersionByID 根据版本ID获取版本
	GetTemplateVersionByID(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error)

	// 供应商

	// BatchCreateTemplateProviders 批量创建模板供应商关联
	BatchCreateTemplateProviders(ctx context.Context, providers []domain.ChannelTemplateProvider) ([]domain.ChannelTemplateProvider, error)
	// GetApprovedProvidersByTemplateIDAndVersionID 根据模版ID和版本ID获取已通过审核的供应商记录
	GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error)
}

// channelTemplateRepository 仓储实现
type channelTemplateRepository struct {
	dao dao.ChannelTemplateDAO
}

// NewChannelTemplateRepository 创建仓储实例
func NewChannelTemplateRepository(dao dao.ChannelTemplateDAO) ChannelTemplateRepository {
	return &channelTemplateRepository{
		dao: dao,
	}
}

// GetTemplates 根据所有者获取模板列表
func (r *channelTemplateRepository) GetTemplates(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error) {
	// 获取模板列表
	templates, err := r.dao.GetTemplatesByOwner(ctx, ownerID, string(ownerType))
	if err != nil {
		return nil, err
	}

	if len(templates) == 0 {
		return []domain.ChannelTemplate{}, nil
	}

	// 提取模板IDs
	templateIDs := make([]int64, len(templates))
	for i := range templates {
		templateIDs[i] = templates[i].ID
	}

	// 获取所有模板关联的版本
	versions, err := r.dao.GetVersionsByTemplateIDs(ctx, templateIDs)
	if err != nil {
		return nil, err
	}

	// 提取版本IDs
	versionIDs := make([]int64, len(versions))
	for i := range versions {
		versionIDs[i] = versions[i].ID
	}

	// 获取所有版本关联的供应商
	providers, err := r.dao.GetProvidersByVersionIDs(ctx, versionIDs)
	if err != nil {
		return nil, err
	}

	// 构建版本ID到供应商列表的映射
	versionToProviders := make(map[int64][]domain.ChannelTemplateProvider)
	for i := range providers {
		domainProvider := r.convertToProviderDomain(providers[i])
		versionToProviders[providers[i].TemplateVersionID] = append(versionToProviders[providers[i].TemplateVersionID], domainProvider)
	}

	// 构建模板ID到版本列表的映射
	templateToVersions := make(map[int64][]domain.ChannelTemplateVersion)
	for i := range versions {
		domainVersion := r.convertToVersionDomain(versions[i])
		// 添加版本关联的供应商
		domainVersion.Providers = versionToProviders[versions[i].ID]
		templateToVersions[versions[i].ChannelTemplateID] = append(templateToVersions[versions[i].ChannelTemplateID], domainVersion)
	}

	// 构建最终的领域模型列表
	result := make([]domain.ChannelTemplate, len(templates))
	for i, t := range templates {
		domainTemplate := r.convertToTemplateDomain(t)
		// 添加模板关联的版本
		domainTemplate.Versions = templateToVersions[t.ID]
		result[i] = domainTemplate
	}

	return result, nil
}

// CreateTemplate 创建模板
func (r *channelTemplateRepository) CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error) {
	// 转换为数据库模型
	daoTemplate := r.convertToTemplateDAO(template)

	// 创建模板
	createdTemplate, err := r.dao.CreateTemplate(ctx, daoTemplate)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}

	// 转换回领域模型
	result := r.convertToTemplateDomain(createdTemplate)
	return result, nil
}

// CreateTemplateVersion 创建模板版本
func (r *channelTemplateRepository) CreateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) (domain.ChannelTemplateVersion, error) {
	// 转换为数据库模型
	daoVersion := r.convertToVersionDAO(version)

	// 创建模板版本
	createdVersion, err := r.dao.CreateTemplateVersion(ctx, daoVersion)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}

	// 转换回领域模型
	result := r.convertToVersionDomain(createdVersion)
	return result, nil
}

// BatchCreateTemplateProviders 批量创建模板供应商关联
func (r *channelTemplateRepository) BatchCreateTemplateProviders(ctx context.Context, providers []domain.ChannelTemplateProvider) ([]domain.ChannelTemplateProvider, error) {
	if len(providers) == 0 {
		return []domain.ChannelTemplateProvider{}, nil
	}

	// 转换为数据库模型
	daoProviders := make([]dao.ChannelTemplateProvider, len(providers))
	for i := range providers {
		daoProviders[i] = r.convertToProviderDAO(providers[i])
	}

	// 批量创建
	createdProviders, err := r.dao.BatchCreateTemplateProviders(ctx, daoProviders)
	if err != nil {
		return nil, err
	}

	// 转换回领域模型
	result := make([]domain.ChannelTemplateProvider, len(createdProviders))
	for i := range createdProviders {
		result[i] = r.convertToProviderDomain(createdProviders[i])
	}

	return result, nil
}

// 数据库模型转领域模型
func (r *channelTemplateRepository) convertToTemplateDomain(daoTemplate dao.ChannelTemplate) domain.ChannelTemplate {
	return domain.ChannelTemplate{
		ID:              daoTemplate.ID,
		OwnerID:         daoTemplate.OwnerID,
		OwnerType:       domain.OwnerType(daoTemplate.OwnerType),
		Name:            daoTemplate.Name,
		Description:     daoTemplate.Description,
		Channel:         domain.Channel(daoTemplate.Channel),
		BusinessType:    domain.BusinessType(daoTemplate.BusinessType),
		ActiveVersionID: daoTemplate.ActiveVersionID,
		Ctime:           daoTemplate.Ctime,
		Utime:           daoTemplate.Utime,
	}
}

func (r *channelTemplateRepository) convertToVersionDomain(daoVersion dao.ChannelTemplateVersion) domain.ChannelTemplateVersion {
	return domain.ChannelTemplateVersion{
		ID:                       daoVersion.ID,
		ChannelTemplateID:        daoVersion.ChannelTemplateID,
		Name:                     daoVersion.Name,
		Signature:                daoVersion.Signature,
		Content:                  daoVersion.Content,
		Remark:                   daoVersion.Remark,
		AuditID:                  daoVersion.AuditID,
		AuditorID:                daoVersion.AuditorID,
		AuditTime:                daoVersion.AuditTime,
		AuditStatus:              domain.AuditStatus(daoVersion.AuditStatus),
		RejectReason:             daoVersion.RejectReason,
		LastReviewSubmissionTime: daoVersion.LastReviewSubmissionTime,
		Ctime:                    daoVersion.Ctime,
		Utime:                    daoVersion.Utime,
	}
}

func (r *channelTemplateRepository) convertToProviderDomain(daoProvider dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
	return domain.ChannelTemplateProvider{
		ID:                       daoProvider.ID,
		TemplateID:               daoProvider.TemplateID,
		TemplateVersionID:        daoProvider.TemplateVersionID,
		ProviderID:               daoProvider.ProviderID,
		ProviderName:             daoProvider.ProviderName,
		ProviderChannel:          domain.Channel(daoProvider.ProviderChannel),
		RequestID:                daoProvider.RequestID,
		ProviderTemplateID:       daoProvider.ProviderTemplateID,
		AuditStatus:              domain.AuditStatus(daoProvider.AuditStatus),
		RejectReason:             daoProvider.RejectReason,
		LastReviewSubmissionTime: daoProvider.LastReviewSubmissionTime,
		Ctime:                    daoProvider.Ctime,
		Utime:                    daoProvider.Utime,
	}
}

// 领域模型转数据库模型
func (r *channelTemplateRepository) convertToTemplateDAO(domainTemplate domain.ChannelTemplate) dao.ChannelTemplate {
	return dao.ChannelTemplate{
		ID:              domainTemplate.ID,
		OwnerID:         domainTemplate.OwnerID,
		OwnerType:       string(domainTemplate.OwnerType),
		Name:            domainTemplate.Name,
		Description:     domainTemplate.Description,
		Channel:         string(domainTemplate.Channel),
		BusinessType:    int64(domainTemplate.BusinessType),
		ActiveVersionID: domainTemplate.ActiveVersionID,
	}
}

func (r *channelTemplateRepository) convertToVersionDAO(domainVersion domain.ChannelTemplateVersion) dao.ChannelTemplateVersion {
	return dao.ChannelTemplateVersion{
		ID:                       domainVersion.ID,
		ChannelTemplateID:        domainVersion.ChannelTemplateID,
		Name:                     domainVersion.Name,
		Signature:                domainVersion.Signature,
		Content:                  domainVersion.Content,
		Remark:                   domainVersion.Remark,
		AuditID:                  domainVersion.AuditID,
		AuditorID:                domainVersion.AuditorID,
		AuditTime:                domainVersion.AuditTime,
		AuditStatus:              string(domainVersion.AuditStatus),
		RejectReason:             domainVersion.RejectReason,
		LastReviewSubmissionTime: domainVersion.LastReviewSubmissionTime,
	}
}

func (r *channelTemplateRepository) convertToProviderDAO(domainProvider domain.ChannelTemplateProvider) dao.ChannelTemplateProvider {
	return dao.ChannelTemplateProvider{
		ID:                       domainProvider.ID,
		TemplateID:               domainProvider.TemplateID,
		TemplateVersionID:        domainProvider.TemplateVersionID,
		ProviderID:               domainProvider.ProviderID,
		ProviderName:             domainProvider.ProviderName,
		ProviderChannel:          string(domainProvider.ProviderChannel),
		RequestID:                domainProvider.RequestID,
		ProviderTemplateID:       domainProvider.ProviderTemplateID,
		AuditStatus:              string(domainProvider.AuditStatus),
		RejectReason:             domainProvider.RejectReason,
		LastReviewSubmissionTime: domainProvider.LastReviewSubmissionTime,
	}
}

// UpdateTemplate 更新模版
func (r *channelTemplateRepository) UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error {
	return r.dao.UpdateTemplate(ctx, r.convertToTemplateDAO(template))
}

func (r *channelTemplateRepository) GetTemplateVersionByID(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error) {
	version, err := r.dao.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}
	return r.convertToVersionDomain(version), nil
}

func (r *channelTemplateRepository) GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error) {
	providers, err := r.dao.GetApprovedProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return nil, err
	}
	results := make([]domain.ChannelTemplateProvider, len(providers))
	for i := range providers {
		results[i] = r.convertToProviderDomain(providers[i])
	}
	return results, nil
}

func (r *channelTemplateRepository) SetActiveVersion(ctx context.Context, templateID, versionID int64) error {
	return r.dao.SetActiveVersion(ctx, templateID, versionID)
}
