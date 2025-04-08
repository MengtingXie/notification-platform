package repository

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"

	"gorm.io/gorm"
)

var ErrProviderNotFound = errors.New("供应商不存在")

// ProviderRepository 供应商仓储接口
type ProviderRepository interface {
	// Create 创建供应商
	Create(ctx context.Context, provider domain.Provider) (domain.Provider, error)
	// Update 更新供应商
	Update(ctx context.Context, provider domain.Provider) error
	// FindByID 根据ID查找供应商
	FindByID(ctx context.Context, id int64) (domain.Provider, error)
	// FindByChannel 查找指定渠道的所有供应商
	FindByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error)
}

type providerRepository struct {
	dao dao.ProviderDAO
}

func NewProviderRepository(d dao.ProviderDAO) ProviderRepository {
	return &providerRepository{dao: d}
}

func (p *providerRepository) Create(ctx context.Context, provider domain.Provider) (domain.Provider, error) {
	created, err := p.dao.Create(ctx, p.toEntity(provider))
	if err != nil {
		return domain.Provider{}, err
	}
	return p.toDomain(created), nil
}

func (p *providerRepository) toDomain(d dao.Provider) domain.Provider {
	return domain.Provider{
		ID:               d.ID,
		Name:             d.Name,
		Code:             d.Code,
		Channel:          domain.Channel(d.Channel),
		Endpoint:         d.Endpoint,
		APIKey:           d.APIKey,
		APISecret:        d.APISecret,
		Weight:           d.Weight,
		QPSLimit:         d.QPSLimit,
		DailyLimit:       d.DailyLimit,
		AuditCallbackURL: d.AuditCallbackURL,
		Status:           domain.Status(d.Status),
	}
}

func (p *providerRepository) toEntity(provider domain.Provider) dao.Provider {
	daoProvider := dao.Provider{
		ID:               provider.ID,
		Name:             provider.Name,
		Code:             provider.Code,
		Channel:          string(provider.Channel),
		Endpoint:         provider.Endpoint,
		APIKey:           provider.APIKey,
		APISecret:        provider.APISecret,
		Weight:           provider.Weight,
		QPSLimit:         provider.QPSLimit,
		DailyLimit:       provider.DailyLimit,
		AuditCallbackURL: provider.AuditCallbackURL,
		Status:           string(provider.Status),
	}
	return daoProvider
}

func (p *providerRepository) Update(ctx context.Context, provider domain.Provider) error {
	return p.dao.Update(ctx, p.toEntity(provider))
}

func (p *providerRepository) FindByID(ctx context.Context, id int64) (domain.Provider, error) {
	provider, err := p.dao.FindByID(ctx, id)
	if err != nil {
		// 处理未找到的情况，转换为领域错误
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Provider{}, fmt.Errorf("%w", ErrProviderNotFound)
		}
		return domain.Provider{}, err
	}
	return p.toDomain(provider), nil
}

func (p *providerRepository) FindByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error) {
	providers, err := p.dao.FindByChannel(ctx, string(channel))
	if err != nil {
		return nil, err
	}

	result := make([]domain.Provider, 0, len(providers))
	for i := range providers {
		result = append(result, p.toDomain(providers[i]))
	}

	return result, nil
}
