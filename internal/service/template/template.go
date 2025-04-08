package template

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
)

var (
	ErrInvalidParameter           = errors.New("参数非法")
	ErrCreateTemplateFailed       = errors.New("创建模版失败")
	ErrTemplateVersionNotApproved = errors.New("模板版本未审核通过")
	ErrProviderTemplateNotFound   = errors.New("供应商模板未找到")
)

// ChannelTemplateService 模板服务接口

//go:generate mockgen -source=./template.go -destination=../../mocks/template.mock.go -package=templatemocks -typed ChannelTemplateService
type ChannelTemplateService interface {
	// 模版

	// GetTemplates 查找模版
	GetTemplates(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error)
	// GetTemplate 获取模版
	GetTemplate(ctx context.Context, templateID, versionID int64, providerName string, channel domain.Channel) (domain.ChannelTemplate, error)
	// GetTemplateByID 根据模版ID获取模版
	GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error)
	// CreateTemplate 创建模板
	CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error)
	// UpdateTemplate 更新模板
	UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error
	// PublishTemplate 发布模板
	PublishTemplate(ctx context.Context, templateID, versionID int64) error

	// 模版版本

	// ForkVersion 基于已有版本创建模版版本
	ForkVersion(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error)
	// UpdateVersion 更新模板版本
	UpdateVersion(ctx context.Context, version domain.ChannelTemplateVersion) error
	// SubmitForInternalReview 提交内部审核
	SubmitForInternalReview(ctx context.Context, versionID int64) error
	// BatchUpdateVersionAuditStatus 批量更新审核状态
	BatchUpdateVersionAuditStatus(ctx context.Context, versions []domain.ChannelTemplateVersion) error

	// 供应商

	// SubmitForProviderReview 提交供应商审核
	SubmitForProviderReview(ctx context.Context, templateID, versionID, providerID int64) error
	// UpdateProviderAuditStatus 更新供应商审核状态
	UpdateProviderAuditStatus(ctx context.Context, requestID, providerTemplateID string) error
}

// templateService 模板服务实现
type templateService struct {
	repo        repository.ChannelTemplateRepository
	providerSvc providersvc.Service
	auditSvc    audit.Service
}

// NewChannelTemplateService 创建模板服务实例
func NewChannelTemplateService(repo repository.ChannelTemplateRepository, providerSvc providersvc.Service, auditSvc audit.Service) ChannelTemplateService {
	return &templateService{
		repo:        repo,
		providerSvc: providerSvc,
		auditSvc:    auditSvc,
	}
}

func (t *templateService) GetTemplates(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error) {
	// 参数校验
	if ownerID <= 0 {
		return nil, fmt.Errorf("%w: 业务方ID必须大于0", ErrInvalidParameter)
	}

	if err := t.isValidateTemplateOwnerType(ownerType); err != nil {
		return nil, err
	}

	// 从仓储层获取模板列表
	templates, err := t.repo.GetTemplates(ctx, ownerID, ownerType)
	if err != nil {
		return nil, fmt.Errorf("获取模板列表失败: %w", err)
	}

	return templates, nil
}

func (t *templateService) GetTemplate(ctx context.Context, templateID, versionID int64, providerName string, channel domain.Channel) (domain.ChannelTemplate, error) {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v, %v, %v, %v", ctx, templateID, versionID, providerName, channel))
}

func (t *templateService) GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error) {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v", ctx, templateID))
}

func (t *templateService) CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error) {
	// 参数校验
	if err := t.isValidateTemplate(template); err != nil {
		return domain.ChannelTemplate{}, err
	}

	// 设置初始状态
	template.ActiveVersionID = 0 // 默认无活跃版本

	// 创建模板
	createdTemplate, err := t.repo.CreateTemplate(ctx, template)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板失败: %w", ErrCreateTemplateFailed, err)
	}

	// 创建模板版本
	version := domain.ChannelTemplateVersion{
		ChannelTemplateID: createdTemplate.ID,
		Name:              "版本名称，比如v1.0.0",
		Signature:         "提前配置好的可用的短信签名或者Email收件人",
		Content:           "模版变量使用${code}格式，也可以没有变量",
		Remark:            "模版使用场景或者用途说明，有利于供应商审核通过",
	}

	createdVersion, err := t.repo.CreateTemplateVersion(ctx, version)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板版本失败: %w", ErrCreateTemplateFailed, err)
	}

	// 为每个供应商创建关联

	// 获取当前渠道的供应商列表
	providers, err := t.providerSvc.GetProvidersByChannel(ctx, template.Channel)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 获取供应商列表失败: %w", ErrCreateTemplateFailed, err)
	}
	if len(providers) == 0 {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 渠道 %s 没有可用的供应商，联系管理员配置供应商", ErrCreateTemplateFailed, template.Channel)
	}

	templateProviders := make([]domain.ChannelTemplateProvider, 0, len(providers))
	for i := range providers {
		templateProvider := domain.ChannelTemplateProvider{
			TemplateID:        createdTemplate.ID,
			TemplateVersionID: createdVersion.ID,
			ProviderID:        providers[i].ID,
			ProviderName:      providers[i].Name,
			ProviderChannel:   domain.Channel(providers[i].Channel),
		}
		templateProviders = append(templateProviders, templateProvider)
	}
	createdProviders, err := t.repo.BatchCreateTemplateProviders(ctx, templateProviders)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板供应商关联失败: %w", ErrCreateTemplateFailed, err)
	}

	// 组合
	createdVersion.Providers = createdProviders
	createdTemplate.Versions = []domain.ChannelTemplateVersion{createdVersion}

	return createdTemplate, nil
}

func (t *templateService) isValidateTemplate(template domain.ChannelTemplate) error {
	if template.OwnerID <= 0 {
		return fmt.Errorf("%w: 所有者ID", ErrInvalidParameter)
	}

	if err := t.isValidateTemplateOwnerType(template.OwnerType); err != nil {
		return err
	}

	if err := t.isValidateTemplateName(template.Name); err != nil {
		return err
	}

	if err := t.isValidateTemplateDescription(template.Description); err != nil {
		return err
	}

	if template.Channel != domain.ChannelSMS &&
		template.Channel != domain.ChannelEmail &&
		template.Channel != domain.ChannelInApp {
		return fmt.Errorf("%w: 渠道类型", ErrInvalidParameter)
	}

	if err := t.isValidateTemplateBusinessType(template.BusinessType); err != nil {
		return err
	}

	return nil
}

func (t *templateService) isValidateTemplateOwnerType(ownerType domain.OwnerType) error {
	if ownerType != domain.OwnerTypePerson &&
		ownerType != domain.OwnerTypeOrganization {
		return fmt.Errorf("%w: 所有者类型", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) isValidateTemplateBusinessType(businessType domain.BusinessType) error {
	if businessType != domain.BusinessTypePromotion &&
		businessType != domain.BusinessTypeNotification &&
		businessType != domain.BusinessTypeVerificationCode {
		return fmt.Errorf("%w: 业务类型", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) isValidateTemplateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: 模板名称", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) isValidateTemplateDescription(description string) error {
	if description == "" {
		return fmt.Errorf("%w: 模板描述", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error {
	if err := t.isValidateTemplateID(template.ID); err != nil {
		return err
	}

	if err := t.isValidateTemplateName(template.Name); err != nil {
		return err
	}

	if err := t.isValidateTemplateDescription(template.Description); err != nil {
		return err
	}

	if err := t.isValidateTemplateBusinessType(template.BusinessType); err != nil {
		return err
	}

	if err := t.repo.UpdateTemplate(ctx, template); err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}

	return nil
}

func (t *templateService) isValidateTemplateID(templateID int64) error {
	if templateID <= 0 {
		return fmt.Errorf("%w: 模板ID必须大于0", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) PublishTemplate(ctx context.Context, templateID, versionID int64) error {
	if err := t.isValidateTemplateID(templateID); err != nil {
		return err
	}

	if err := t.isValidateTemplateVersionID(versionID); err != nil {
		return err
	}

	// 检查版本是否存在并且已通过内部审核
	version, err := t.repo.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return err
	}

	// 确认版本属于该模板
	if version.ChannelTemplateID != templateID {
		return fmt.Errorf("%w: 模版ID与版本ID不关联", ErrInvalidParameter)
	}

	// 检查版本是否通过内部审核
	if version.AuditStatus != "APPROVED" {
		return fmt.Errorf("%w: 版本ID", ErrInvalidParameter)
	}

	// 检查是否有通过供应商审核的记录
	providers, err := t.repo.GetApprovedProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return fmt.Errorf("%w", ErrTemplateVersionNotApproved)
	}

	// 设置活跃版本
	if err := t.repo.SetActiveVersion(ctx, templateID, versionID); err != nil {
		return fmt.Errorf("发布模版失败: %w", err)
	}
	return nil
}

func (t *templateService) isValidateTemplateVersionID(versionID int64) error {
	if versionID <= 0 {
		return fmt.Errorf("%w: 版本ID必须大于0", ErrInvalidParameter)
	}
	return nil
}

func (t *templateService) ForkVersion(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error) {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v", ctx, versionID))
}

func (t *templateService) UpdateVersion(ctx context.Context, version domain.ChannelTemplateVersion) error {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v, %v", ctx, version))
}

func (t *templateService) BatchUpdateVersionAuditStatus(ctx context.Context, versions []domain.ChannelTemplateVersion) error {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v", ctx, versions))
}

func (t *templateService) SubmitForInternalReview(ctx context.Context, versionID int64) error {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v", ctx, versionID))
}

func (t *templateService) SubmitForProviderReview(ctx context.Context, templateID, versionID, providerID int64) error {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v %v %v", ctx, templateID, versionID, providerID))
}

func (t *templateService) UpdateProviderAuditStatus(ctx context.Context, requestID, providerTemplateID string) error {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v %v %v", ctx, requestID, providerTemplateID))
}
