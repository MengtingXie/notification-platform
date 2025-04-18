package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/slice"
	"golang.org/x/sync/errgroup"
)

// ChannelTemplateRepository 提供模板数据存储的仓库接口
type ChannelTemplateRepository interface {
	// 模版相关方法

	// GetTemplatesByOwner 获取指定所有者的模板列表
	GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error)

	// GetTemplateByID 根据ID获取模板
	GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error)

	// CreateTemplate 创建模板
	CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error)

	// UpdateTemplate 更新模板
	UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error

	// SetTemplateActiveVersion 设置模板的活跃版本
	SetTemplateActiveVersion(ctx context.Context, templateID, versionID int64) error

	// 模版版本相关方法

	// GetTemplateVersionByID 根据ID获取模板版本
	GetTemplateVersionByID(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error)

	// CreateTemplateVersion 创建模板版本
	CreateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) (domain.ChannelTemplateVersion, error)

	// ForkTemplateVersion 基于已有版本创建新版本
	ForkTemplateVersion(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error)

	// 供应商相关方法

	// GetProviderByNameAndChannel 根据名称和渠道获取供应商
	GetProviderByNameAndChannel(ctx context.Context, templateID, versionID int64, providerName string, channel domain.Channel) ([]domain.ChannelTemplateProvider, error)

	// BatchCreateTemplateProviders 批量创建模板供应商关联
	BatchCreateTemplateProviders(ctx context.Context, providers []domain.ChannelTemplateProvider) ([]domain.ChannelTemplateProvider, error)

	// GetApprovedProvidersByTemplateIDAndVersionID 获取已审核通过的供应商列表
	GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error)

	// GetProvidersByTemplateIDAndVersionID 获取模板和版本关联的所有供应商
	GetProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error)

	// UpdateTemplateVersion 更新模板版本
	UpdateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) error

	// BatchUpdateTemplateVersionAuditInfo 批量更新模板版本审核信息
	BatchUpdateTemplateVersionAuditInfo(ctx context.Context, versions []domain.ChannelTemplateVersion) error

	// UpdateTemplateProviderAuditInfo 更新模板供应商审核信息
	UpdateTemplateProviderAuditInfo(ctx context.Context, provider domain.ChannelTemplateProvider) error

	// BatchUpdateTemplateProvidersAuditInfo 批量更新模板供应商审核信息
	BatchUpdateTemplateProvidersAuditInfo(ctx context.Context, providers []domain.ChannelTemplateProvider) error

	// GetPendingOrInReviewProviders 获取未审核或审核中的供应商关联
	GetPendingOrInReviewProviders(ctx context.Context, offset, limit int, ctime int64) (providers []domain.ChannelTemplateProvider, total int64, err error)
}

// channelTemplateRepository 实现了ChannelTemplateRepository接口，提供模板数据的存储实现
type channelTemplateRepository struct {
	dao dao.ChannelTemplateDAO
}

// NewChannelTemplateRepository 创建仓储实例
func NewChannelTemplateRepository(dao dao.ChannelTemplateDAO) ChannelTemplateRepository {
	return &channelTemplateRepository{
		dao: dao,
	}
}

// 模版相关方法

func (r *channelTemplateRepository) GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error) {
	// 获取模板列表
	templates, err := r.dao.GetTemplatesByOwner(ctx, ownerID, ownerType.String())
	if err != nil {
		return nil, err
	}

	if len(templates) == 0 {
		return []domain.ChannelTemplate{}, nil
	}

	return r.getTemplates(ctx, templates)
}

func (r *channelTemplateRepository) getTemplates(ctx context.Context, templates []dao.ChannelTemplate) ([]domain.ChannelTemplate, error) {
	// 提取模板IDs
	templateIDs := make([]int64, len(templates))
	for i := range templates {
		templateIDs[i] = templates[i].ID
	}

	// 获取所有模板关联的版本
	versions, err := r.dao.GetTemplateVersionsByTemplateIDs(ctx, templateIDs)
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
		domainProvider := r.toProviderDomain(providers[i])
		versionToProviders[providers[i].TemplateVersionID] = append(versionToProviders[providers[i].TemplateVersionID], domainProvider)
	}

	// 构建模板ID到版本列表的映射
	templateToVersions := make(map[int64][]domain.ChannelTemplateVersion)
	for i := range versions {
		domainVersion := r.toVersionDomain(versions[i])
		// 添加版本关联的供应商
		domainVersion.Providers = versionToProviders[versions[i].ID]
		templateToVersions[versions[i].ChannelTemplateID] = append(templateToVersions[versions[i].ChannelTemplateID], domainVersion)
	}

	// 构建最终的领域模型列表
	result := make([]domain.ChannelTemplate, len(templates))
	for i, t := range templates {
		domainTemplate := r.toTemplateDomain(t)
		// 添加模板关联的版本
		domainTemplate.Versions = templateToVersions[t.ID]
		result[i] = domainTemplate
	}

	return result, nil
}

func (r *channelTemplateRepository) GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error) {
	templateEntity, err := r.dao.GetTemplateByID(ctx, templateID)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}
	templates, err := r.getTemplates(ctx, []dao.ChannelTemplate{templateEntity})
	if err != nil {
		return domain.ChannelTemplate{}, err
	}
	const first = 0
	return templates[first], nil
}

func (r *channelTemplateRepository) CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error) {
	// 转换为数据库模型
	daoTemplate := r.toTemplateEntity(template)

	// 创建模板
	createdTemplate, err := r.dao.CreateTemplate(ctx, daoTemplate)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}

	// 转换回领域模型
	result := r.toTemplateDomain(createdTemplate)
	return result, nil
}

func (r *channelTemplateRepository) UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error {
	return r.dao.UpdateTemplate(ctx, r.toTemplateEntity(template))
}

func (r *channelTemplateRepository) SetTemplateActiveVersion(ctx context.Context, templateID, versionID int64) error {
	return r.dao.SetTemplateActiveVersion(ctx, templateID, versionID)
}

// 模版版本相关方法

func (r *channelTemplateRepository) GetTemplateVersionByID(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error) {
	version, err := r.dao.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}
	providers, err := r.dao.GetProvidersByVersionIDs(ctx, []int64{versionID})
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}
	domainProviders := make([]domain.ChannelTemplateProvider, 0, len(providers))
	for i := range providers {
		domainProviders = append(domainProviders, r.toProviderDomain(providers[i]))
	}

	domainVersion := r.toVersionDomain(version)
	domainVersion.Providers = domainProviders
	return domainVersion, nil
}

// CreateTemplateVersion 创建模板版本
func (r *channelTemplateRepository) CreateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) (domain.ChannelTemplateVersion, error) {
	// 转换为数据库模型
	daoVersion := r.toVersionEntity(version)

	// 创建模板版本
	createdVersion, err := r.dao.CreateTemplateVersion(ctx, daoVersion)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}

	// 转换回领域模型
	result := r.toVersionDomain(createdVersion)
	return result, nil
}

func (r *channelTemplateRepository) ForkTemplateVersion(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error) {
	v, err := r.dao.ForkTemplateVersion(ctx, versionID)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}

	version := r.toVersionDomain(v)

	providers, err := r.dao.GetProvidersByTemplateIDAndVersionID(ctx, v.ChannelTemplateID, v.ID)
	if err != nil {
		return domain.ChannelTemplateVersion{}, err
	}

	version.Providers = slice.Map(providers, func(_ int, src dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
		return r.toProviderDomain(src)
	})

	return version, nil
}

// 供应商相关方法

func (r *channelTemplateRepository) GetProviderByNameAndChannel(ctx context.Context, templateID, versionID int64, providerName string, channel domain.Channel) ([]domain.ChannelTemplateProvider, error) {
	providers, err := r.dao.GetProviderByNameAndChannel(ctx, templateID, versionID, providerName, channel.String())
	if err != nil {
		return nil, err
	}
	results := make([]domain.ChannelTemplateProvider, len(providers))
	for i := range providers {
		results[i] = r.toProviderDomain(providers[i])
	}
	return results, nil
}

// BatchCreateTemplateProviders 批量创建模板供应商关联
func (r *channelTemplateRepository) BatchCreateTemplateProviders(ctx context.Context, providers []domain.ChannelTemplateProvider) ([]domain.ChannelTemplateProvider, error) {
	if len(providers) == 0 {
		return []domain.ChannelTemplateProvider{}, nil
	}

	// 转换为数据库模型
	daoProviders := slice.Map(providers, func(_ int, src domain.ChannelTemplateProvider) dao.ChannelTemplateProvider {
		return r.toProviderEntity(src)
	})

	// 批量创建
	createdProviders, err := r.dao.BatchCreateTemplateProviders(ctx, daoProviders)
	if err != nil {
		return nil, err
	}

	// 转换回领域模型
	return slice.Map(createdProviders, func(_ int, src dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
		return r.toProviderDomain(src)
	}), nil
}

func (r *channelTemplateRepository) GetApprovedProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error) {
	providers, err := r.dao.GetApprovedProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return nil, err
	}
	return slice.Map(providers, func(_ int, src dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
		return r.toProviderDomain(src)
	}), nil
}

func (r *channelTemplateRepository) GetProvidersByTemplateIDAndVersionID(ctx context.Context, templateID, versionID int64) ([]domain.ChannelTemplateProvider, error) {
	providers, err := r.dao.GetProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return nil, err
	}
	return slice.Map(providers, func(_ int, src dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
		return r.toProviderDomain(src)
	}), nil
}

func (r *channelTemplateRepository) UpdateTemplateVersion(ctx context.Context, version domain.ChannelTemplateVersion) error {
	return r.dao.UpdateTemplateVersion(ctx, r.toVersionEntity(version))
}

func (r *channelTemplateRepository) BatchUpdateTemplateVersionAuditInfo(ctx context.Context, versions []domain.ChannelTemplateVersion) error {
	return r.dao.BatchUpdateTemplateVersionAuditInfo(ctx, slice.Map(versions, func(_ int, src domain.ChannelTemplateVersion) dao.ChannelTemplateVersion {
		return r.toVersionEntity(src)
	}))
}

func (r *channelTemplateRepository) UpdateTemplateProviderAuditInfo(ctx context.Context, provider domain.ChannelTemplateProvider) error {
	return r.dao.UpdateTemplateProviderAuditInfo(ctx, r.toProviderEntity(provider))
}

func (r *channelTemplateRepository) BatchUpdateTemplateProvidersAuditInfo(ctx context.Context, providers []domain.ChannelTemplateProvider) error {
	daoProviders := slice.Map(providers, func(_ int, src domain.ChannelTemplateProvider) dao.ChannelTemplateProvider {
		return r.toProviderEntity(src)
	})
	return r.dao.BatchUpdateTemplateProvidersAuditInfo(ctx, daoProviders)
}

// GetPendingOrInReviewProviders 获取未审核或审核中的供应商关联
func (r *channelTemplateRepository) GetPendingOrInReviewProviders(ctx context.Context, offset, limit int, utime int64) (providers []domain.ChannelTemplateProvider, total int64, err error) {
	var (
		eg           errgroup.Group
		daoProviders []dao.ChannelTemplateProvider
	)
	eg.Go(func() error {
		var err1 error
		daoProviders, err1 = r.dao.GetPendingOrInReviewProviders(ctx, offset, limit, utime)
		return err1
	})

	eg.Go(func() error {
		var err2 error
		total, err2 = r.dao.TotalPendingOrInReviewProviders(ctx, utime)
		return err2
	})

	return slice.Map(daoProviders, func(_ int, src dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
		return r.toProviderDomain(src)
	}), total, eg.Wait()
}

func (r *channelTemplateRepository) toTemplateDomain(daoTemplate dao.ChannelTemplate) domain.ChannelTemplate {
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

func (r *channelTemplateRepository) toVersionDomain(daoVersion dao.ChannelTemplateVersion) domain.ChannelTemplateVersion {
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

func (r *channelTemplateRepository) toProviderDomain(daoProvider dao.ChannelTemplateProvider) domain.ChannelTemplateProvider {
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

func (r *channelTemplateRepository) toTemplateEntity(domainTemplate domain.ChannelTemplate) dao.ChannelTemplate {
	return dao.ChannelTemplate{
		ID:              domainTemplate.ID,
		OwnerID:         domainTemplate.OwnerID,
		OwnerType:       domainTemplate.OwnerType.String(),
		Name:            domainTemplate.Name,
		Description:     domainTemplate.Description,
		Channel:         domainTemplate.Channel.String(),
		BusinessType:    domainTemplate.BusinessType.ToInt64(),
		ActiveVersionID: domainTemplate.ActiveVersionID,
	}
}

func (r *channelTemplateRepository) toVersionEntity(domainVersion domain.ChannelTemplateVersion) dao.ChannelTemplateVersion {
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
		AuditStatus:              domainVersion.AuditStatus.String(),
		RejectReason:             domainVersion.RejectReason,
		LastReviewSubmissionTime: domainVersion.LastReviewSubmissionTime,
	}
}

func (r *channelTemplateRepository) toProviderEntity(domainProvider domain.ChannelTemplateProvider) dao.ChannelTemplateProvider {
	return dao.ChannelTemplateProvider{
		ID:                       domainProvider.ID,
		TemplateID:               domainProvider.TemplateID,
		TemplateVersionID:        domainProvider.TemplateVersionID,
		ProviderID:               domainProvider.ProviderID,
		ProviderName:             domainProvider.ProviderName,
		ProviderChannel:          domainProvider.ProviderChannel.String(),
		RequestID:                domainProvider.RequestID,
		ProviderTemplateID:       domainProvider.ProviderTemplateID,
		AuditStatus:              domainProvider.AuditStatus.String(),
		RejectReason:             domainProvider.RejectReason,
		LastReviewSubmissionTime: domainProvider.LastReviewSubmissionTime,
	}
}
