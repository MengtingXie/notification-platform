package manage

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/audit"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
)

// ChannelTemplateService 模板服务接口
//
//go:generate mockgen -source=./manage.go -destination=../mocks/manage.mock.go -package=templatemocks -typed ChannelTemplateService
type ChannelTemplateService interface {
	// 模版

	// GetTemplatesByOwner DONE
	GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error)

	// GetTemplateByIDAndProviderInfo DONE
	GetTemplateByIDAndProviderInfo(ctx context.Context, templateID int64, providerName string, channel domain.Channel) (domain.ChannelTemplate, error)

	// GetTemplateByID DONE
	GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error)

	// CreateTemplate DONE
	CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error)

	// UpdateTemplate DONE
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
	SubmitForProviderReview(ctx context.Context, templateID, versionID int64) error
	// UpdateProviderAuditStatus 更新供应商审核状态
	UpdateProviderAuditStatus(ctx context.Context, requestID, providerTemplateID string) error
}

// templateService 模板服务实现
type templateService struct {
	repo        repository.ChannelTemplateRepository
	providerSvc providersvc.Service
	auditSvc    audit.Service
	smsClients  map[string]client.Client
}

// NewChannelTemplateService 创建模板服务实例
func NewChannelTemplateService(
	repo repository.ChannelTemplateRepository,
	providerSvc providersvc.Service,
	auditSvc audit.Service,
	smsClients map[string]client.Client,
) ChannelTemplateService {
	return &templateService{
		repo:        repo,
		providerSvc: providerSvc,
		auditSvc:    auditSvc,
		smsClients:  smsClients,
	}
}

// 模版相关方法

func (t *templateService) GetTemplatesByOwner(ctx context.Context, ownerID int64, ownerType domain.OwnerType) ([]domain.ChannelTemplate, error) {
	if ownerID <= 0 {
		return nil, fmt.Errorf("%w: 业务方ID必须大于0", errs.ErrInvalidParameter)
	}

	if !ownerType.IsValid() {
		return nil, fmt.Errorf("%w: 所有者类型", errs.ErrInvalidParameter)
	}

	// 从仓储层获取模板列表
	templates, err := t.repo.GetTemplatesByOwner(ctx, ownerID, ownerType)
	if err != nil {
		return nil, fmt.Errorf("获取模板列表失败: %w", err)
	}

	return templates, nil
}

func (t *templateService) GetTemplateByIDAndProviderInfo(ctx context.Context, templateID int64, providerName string, channel domain.Channel) (domain.ChannelTemplate, error) {
	// 1. 获取模板基本信息
	template, err := t.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}

	if template.ID == 0 {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: templateID=%d", errs.ErrTemplateNotFound, templateID)
	}

	// 2. 获取指定的版本信息
	version, err := t.repo.GetTemplateVersionByID(ctx, template.ActiveVersionID)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}

	if version.AuditStatus != domain.AuditStatusApproved {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: versionID=%d", errs.ErrTemplateVersionNotApprovedByPlatform, version.ID)
	}

	// 3. 获取指定供应商信息
	providers, err := t.repo.GetProviderByNameAndChannel(ctx, templateID, version.ID, providerName, channel)
	if err != nil {
		return domain.ChannelTemplate{}, err
	}

	if len(providers) == 0 {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: providerName=%s, channel=%s", errs.ErrProviderNotFound, providerName, channel)
	}

	// 4. 组装完整模板
	version.Providers = providers
	template.Versions = []domain.ChannelTemplateVersion{version}

	return template, nil
}

func (t *templateService) GetTemplateByID(ctx context.Context, templateID int64) (domain.ChannelTemplate, error) {
	return t.repo.GetTemplateByID(ctx, templateID)
}

func (t *templateService) CreateTemplate(ctx context.Context, template domain.ChannelTemplate) (domain.ChannelTemplate, error) {
	// 参数校验
	if err := template.Validate(); err != nil {
		return domain.ChannelTemplate{}, err
	}

	// 设置初始状态
	template.ActiveVersionID = 0 // 默认无活跃版本

	// 创建模板
	createdTemplate, err := t.repo.CreateTemplate(ctx, template)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板失败: %w", errs.ErrCreateTemplateFailed, err)
	}

	// 创建模板版本，填充伪数据
	version := domain.ChannelTemplateVersion{
		ChannelTemplateID: createdTemplate.ID,
		Name:              "版本名称，比如v1.0.0",
		Signature:         "提前配置好的可用的短信签名或者Email收件人",
		Content:           "模版变量使用${code}格式，也可以没有变量",
		Remark:            "模版使用场景或者用途说明，有利于供应商审核通过",
	}

	createdVersion, err := t.repo.CreateTemplateVersion(ctx, version)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板版本失败: %w", errs.ErrCreateTemplateFailed, err)
	}

	// 为每个供应商创建关联
	providers, err := t.providerSvc.GetByChannel(ctx, template.Channel)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 获取供应商列表失败: %w", errs.ErrCreateTemplateFailed, err)
	}
	if len(providers) == 0 {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 渠道 %s 没有可用的供应商，联系管理员配置供应商", errs.ErrCreateTemplateFailed, template.Channel)
	}
	templateProviders := make([]domain.ChannelTemplateProvider, 0, len(providers))
	for i := range providers {
		templateProvider := domain.ChannelTemplateProvider{
			TemplateID:        createdTemplate.ID,
			TemplateVersionID: createdVersion.ID,
			ProviderID:        providers[i].ID,
			ProviderName:      providers[i].Name,
			ProviderChannel:   providers[i].Channel,
		}
		templateProviders = append(templateProviders, templateProvider)
	}
	createdProviders, err := t.repo.BatchCreateTemplateProviders(ctx, templateProviders)
	if err != nil {
		return domain.ChannelTemplate{}, fmt.Errorf("%w: 创建模板供应商关联失败: %w", errs.ErrCreateTemplateFailed, err)
	}

	// 组合
	createdVersion.Providers = createdProviders
	createdTemplate.Versions = []domain.ChannelTemplateVersion{createdVersion}
	return createdTemplate, nil
}

// UpdateTemplate 更新模版的基础信息
func (t *templateService) UpdateTemplate(ctx context.Context, template domain.ChannelTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("%w: 模板名称", errs.ErrInvalidParameter)
	}

	if template.Description == "" {
		return fmt.Errorf("%w: 模板描述", errs.ErrInvalidParameter)
	}

	if !template.BusinessType.IsValid() {
		return fmt.Errorf("%w: 业务类型", errs.ErrInvalidParameter)
	}

	if err := t.repo.UpdateTemplate(ctx, template); err != nil {
		return fmt.Errorf("%w: %w", errs.ErrUpdateTemplateFailed, err)
	}

	return nil
}

func (t *templateService) PublishTemplate(ctx context.Context, templateID, versionID int64) error {
	if templateID <= 0 {
		return fmt.Errorf("%w: 模板ID必须大于0", errs.ErrInvalidParameter)
	}

	if versionID <= 0 {
		return fmt.Errorf("%w: 版本ID必须大于0", errs.ErrInvalidParameter)
	}

	// 检查版本是否存在并且已通过内部审核
	version, err := t.repo.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return err
	}

	// 确认版本属于该模板
	if version.ChannelTemplateID != templateID {
		return fmt.Errorf("%w: %w", errs.ErrInvalidParameter, errs.ErrTemplateAndVersionMisMatch)
	}

	// 检查版本是否通过内部审核
	if version.AuditStatus != domain.AuditStatusApproved {
		return fmt.Errorf("%w: %w: 版本ID", errs.ErrInvalidParameter, errs.ErrTemplateVersionNotApprovedByPlatform)
	}

	// 检查是否有通过供应商审核的记录
	providers, err := t.repo.GetApprovedProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return fmt.Errorf("%w", errs.ErrTemplateVersionNotApprovedByPlatform)
	}

	// 设置活跃版本
	if err := t.repo.SetTemplateActiveVersion(ctx, templateID, versionID); err != nil {
		return fmt.Errorf("发布模版失败: %w", err)
	}
	return nil
}

// 模版版本相关方法

func (t *templateService) ForkVersion(ctx context.Context, versionID int64) (domain.ChannelTemplateVersion, error) {
	return t.repo.ForkTemplateVersion(ctx, versionID)
}

func (t *templateService) UpdateVersion(ctx context.Context, version domain.ChannelTemplateVersion) error {
	// 参数校验
	if version.ID <= 0 {
		return fmt.Errorf("%w: 版本ID必须大于0", errs.ErrInvalidParameter)
	}

	// 获取当前版本
	currentVersion, err := t.repo.GetTemplateVersionByID(ctx, version.ID)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrUpdateTemplateVersionFailed, err)
	}

	// 检查版本状态，只有PENDING或REJECTED状态的版本才能修改
	if currentVersion.AuditStatus != domain.AuditStatusPending && currentVersion.AuditStatus != domain.AuditStatusRejected {
		return fmt.Errorf("%w: %w: 只有待审核或拒绝状态的版本可以修改", errs.ErrUpdateTemplateVersionFailed, errs.ErrInvalidOperation)
	}

	// 允许更新部分字段
	updateVersion := domain.ChannelTemplateVersion{
		ID:        version.ID,
		Name:      version.Name,
		Signature: version.Signature,
		Content:   version.Content,
		Remark:    version.Remark,
	}

	// 更新版本
	if err1 := t.repo.UpdateTemplateVersion(ctx, updateVersion); err1 != nil {
		return fmt.Errorf("%w: %w", errs.ErrUpdateTemplateVersionFailed, err1)
	}

	return nil
}

func (t *templateService) BatchUpdateVersionAuditStatus(ctx context.Context, versions []domain.ChannelTemplateVersion) error {
	if len(versions) == 0 {
		return nil
	}
	if err := t.repo.BatchUpdateTemplateVersionAuditStatus(ctx, versions); err != nil {
		return fmt.Errorf("%w: %w", errs.ErrUpdateTemplateVersionAuditStatusFailed, err)
	}
	return nil
}

func (t *templateService) SubmitForInternalReview(ctx context.Context, versionID int64) error {
	if versionID <= 0 {
		return fmt.Errorf("%w: 版本ID必须大于0", errs.ErrInvalidParameter)
	}

	// 获取版本信息
	version, err := t.repo.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return err
	}

	// 获取模板信息
	template, err := t.repo.GetTemplateByID(ctx, version.ChannelTemplateID)
	if err != nil {
		return err
	}

	// 获取版本关联的供应商
	providers, err := t.repo.GetProvidersByTemplateIDAndVersionID(ctx, template.ID, version.ID)
	if err != nil {
		return err
	}

	// 构建审核内容
	auditData := map[string]any{
		"template":  template,
		"version":   version,
		"providers": providers,
	}

	// 创建审核记录
	auditReq := domain.Audit{
		ResourceID:   version.ID,
		ResourceType: domain.ResourceTypeTemplate,
		Content:      fmt.Sprintf("%v", auditData), // 简化处理，实际应该是JSON格式
	}

	auditID, err := t.auditSvc.CreateAudit(ctx, auditReq)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForInternalReviewFailed, err)
	}

	// 更新版本审核状态
	now := time.Now().Unix()
	updateVersions := []domain.ChannelTemplateVersion{
		{
			ID:                       version.ID,
			AuditID:                  auditID,
			AuditStatus:              domain.AuditStatusInReview,
			LastReviewSubmissionTime: now,
		},
	}

	if err := t.repo.BatchUpdateTemplateVersionAuditStatus(ctx, updateVersions); err != nil {
		return fmt.Errorf("%w: 更新版本审核状态失败: %w", errs.ErrSubmitVersionForInternalReviewFailed, err)
	}

	return nil
}

// 供应商相关方法

func (t *templateService) SubmitForProviderReview(ctx context.Context, templateID, versionID int64) error {
	if templateID <= 0 {
		return fmt.Errorf("%w: 模版ID必须大于0", errs.ErrInvalidParameter)
	}

	if versionID <= 0 {
		return fmt.Errorf("%w: 模版版本ID必须大于0", errs.ErrInvalidParameter)
	}

	// 获取版本信息
	version, err := t.repo.GetTemplateVersionByID(ctx, versionID)
	if err != nil {
		return err
	}

	// 确认版本属于该模板
	if version.ChannelTemplateID != templateID {
		return fmt.Errorf("%w: %w", errs.ErrInvalidParameter, errs.ErrTemplateAndVersionMisMatch)
	}

	// 检查版本是否通过内部审核
	if version.AuditStatus != domain.AuditStatusApproved {
		return fmt.Errorf("%w: %w: 版本未通过内部审核", errs.ErrSubmitVersionForProviderReviewFailed, errs.ErrInvalidOperation)
	}

	// 获取模板信息
	template, err := t.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForProviderReviewFailed, err)
	}

	// 获取供应商关联信息
	providers, err := t.repo.GetProvidersByTemplateIDAndVersionID(ctx, templateID, versionID)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForProviderReviewFailed, err)
	}

	for i := range providers {
		if providers[i].AuditStatus == domain.AuditStatusPending ||
			providers[i].AuditStatus == domain.AuditStatusRejected {
			_ = t.submitForProviderReview(ctx, template, version, providers[i])
		}
	}
	return nil
}

func (t *templateService) submitForProviderReview(ctx context.Context, template domain.ChannelTemplate, version domain.ChannelTemplateVersion, provider domain.ChannelTemplateProvider) error {
	// 当前仅支持SMS渠道
	if provider.ProviderChannel != domain.ChannelSMS {
		return nil
	}
	// 获取对应的SMS客户端
	cli, err := t.getSMSClient(provider.ProviderName)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForProviderReviewFailed, err)
	}

	// 构建供应商审核请求并调用
	content, err := t.replacePlaceholders(ctx, version.Content, provider)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForProviderReviewFailed, err)
	}
	resp, err := cli.CreateTemplate(client.CreateTemplateReq{
		TemplateName:    version.Name,
		TemplateContent: content,
		TemplateType:    client.TemplateType(template.BusinessType),
		Remark:          version.Remark,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrSubmitVersionForProviderReviewFailed, err)
	}

	// 更新供应商关联
	if err1 := t.repo.UpdateTemplateProvider(ctx, (domain.ChannelTemplateProvider{
		ID:                       provider.ID,
		RequestID:                resp.RequestID,
		AuditStatus:              domain.AuditStatusInReview,
		LastReviewSubmissionTime: time.Now().Unix(),
	})); err1 != nil {
		return fmt.Errorf("%w: 更新供应商关联失败: %w", errs.ErrSubmitVersionForProviderReviewFailed, err1)
	}
	return nil
}

func (t *templateService) getSMSClient(providerName string) (client.Client, error) {
	smsClient, ok := t.smsClients[providerName]
	if !ok {
		return nil, fmt.Errorf("未找到对应的供应商客户端")
	}
	return smsClient, nil
}

func (t *templateService) replacePlaceholders(ctx context.Context, content string, provider domain.ChannelTemplateProvider) (string, error) {
	p, err := t.providerSvc.GetByID(ctx, provider.ProviderID)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errs.ErrProviderNotFound, err)
	}

	if p.TemplateRegExp == "" {
		return content, nil
	}

	// 如果配置了替换规则
	re := regexp.MustCompile(`\$\{[^}]+\}`)
	counter := 0
	output := re.ReplaceAllStringFunc(content, func(_ string) string {
		counter++
		return fmt.Sprintf(p.TemplateRegExp, counter)
	})
	return output, nil
}

func (t *templateService) UpdateProviderAuditStatus(ctx context.Context, requestID, providerTemplateID string) error {
	if requestID == "" || providerTemplateID == "" {
		return fmt.Errorf("%w: 参数不能为空", errs.ErrInvalidParameter)
	}

	// 根据请求ID获取供应商关联
	provider, err := t.repo.GetProviderByRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("%w: 获取供应商关联失败: %w", errs.ErrUpdateTemplateProviderAuditStatusFailed, err)
	}

	if provider.ID == 0 {
		return fmt.Errorf("%w: %w: 未找到对应的供应商关联", errs.ErrUpdateTemplateProviderAuditStatusFailed, errs.ErrProviderNotFound)
	}

	// 获取对应的SMS客户端
	cli, err := t.getSMSClient(provider.ProviderName)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrUpdateTemplateProviderAuditStatusFailed, err)
	}

	// 查询审核状态
	resp, err := cli.QueryTemplateStatus(client.QueryTemplateStatusReq{
		TemplateID: providerTemplateID,
	})
	if err != nil {
		return fmt.Errorf("%w: 查询供应商审核状态失败: %w", errs.ErrUpdateTemplateProviderAuditStatusFailed, err)
	}

	// 转换审核状态
	var auditStatus domain.AuditStatus

	switch {
	case resp.AuditStatus.IsPending():
		auditStatus = domain.AuditStatusInReview
	case resp.AuditStatus.IsApproved():
		auditStatus = domain.AuditStatusApproved
	case resp.AuditStatus.IsRejected():
		auditStatus = domain.AuditStatusRejected
	default:
		auditStatus = domain.AuditStatusPending
	}

	// 更新供应商关联
	updateProvider := domain.ChannelTemplateProvider{
		ID:                 provider.ID,
		ProviderTemplateID: providerTemplateID,
		AuditStatus:        auditStatus,
		RejectReason:       resp.Reason,
	}

	if err := t.repo.UpdateTemplateProvider(ctx, updateProvider); err != nil {
		return fmt.Errorf("更新供应商关联失败: %w", err)
	}

	return nil
}
