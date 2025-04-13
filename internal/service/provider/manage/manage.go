package manage

import (
	"context"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// Service 供应商服务接口
//
//go:generate mockgen -source=./manage.go -destination=../mocks/manage.mock.go -package=providermocks -typed Service
type Service interface {
	// Create 创建供应商
	Create(ctx context.Context, provider domain.Provider) (domain.Provider, error)
	// Update 更新供应商
	Update(ctx context.Context, provider domain.Provider) error
	// GetByID 根据ID获取供应商
	GetByID(ctx context.Context, id int64) (domain.Provider, error)
	// GetByChannel 获取指定渠道的所有供应商
	GetByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error)
}

// providerService 供应商服务实现
type providerService struct {
	repo repository.ProviderRepository
}

// NewProviderService 创建供应商服务
func NewProviderService(repo repository.ProviderRepository) Service {
	return &providerService{
		repo: repo,
	}
}

// Create 创建供应商
func (s *providerService) Create(ctx context.Context, provider domain.Provider) (domain.Provider, error) {
	if err := provider.Validate(); err != nil {
		return domain.Provider{}, err
	}
	return s.repo.Create(ctx, provider)
}

// Update 更新供应商
func (s *providerService) Update(ctx context.Context, provider domain.Provider) error {
	if err := provider.Validate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, provider)
}

// GetByID 根据ID获取供应商
func (s *providerService) GetByID(ctx context.Context, id int64) (domain.Provider, error) {
	if id <= 0 {
		return domain.Provider{}, fmt.Errorf("%w: 供应商ID必须大于0", errs.ErrInvalidParameter)
	}
	return s.repo.FindByID(ctx, id)
}

// GetByChannel 获取指定渠道的所有供应商
func (s *providerService) GetByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error) {
	if !channel.IsValid() {
		return nil, fmt.Errorf("%w: 不支持的渠道类型", errs.ErrInvalidParameter)
	}
	return s.repo.FindByChannel(ctx, channel)
}
