package manage

import (
	"context"
	"errors"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
)

var (
	ErrProviderNotFound = repository.ErrProviderNotFound
	ErrInvalidParameter = errors.New("参数非法")
)

// Service 供应商服务接口
//
//go:generate mockgen -source=./manage.go -destination=../mocks/manage.mock.go -package=providermocks -typed Service
type Service interface {
	// CreateProvider 创建供应商
	CreateProvider(ctx context.Context, provider domain.Provider) (domain.Provider, error)
	// UpdateProvider 更新供应商
	UpdateProvider(ctx context.Context, provider domain.Provider) error
	// GetProviderByID 根据ID获取供应商
	GetProviderByID(ctx context.Context, id int64) (domain.Provider, error)
	// GetProviderIDByNameAndChannel 获取供应商ID
	GetProviderIDByNameAndChannel(ctx context.Context, name string, channel domain.Channel) (domain.Provider, error)
	// GetProvidersByChannel 获取指定渠道的所有供应商
	GetProvidersByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error)
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

// CreateProvider 创建供应商
func (s *providerService) CreateProvider(ctx context.Context, provider domain.Provider) (domain.Provider, error) {
	if err := s.isValidateProvider(provider); err != nil {
		return domain.Provider{}, err
	}
	return s.repo.Create(ctx, provider)
}

// isValidateProvider 验证供应商参数
func (s *providerService) isValidateProvider(provider domain.Provider) error {
	if provider.Name == "" {
		return fmt.Errorf("%w: 供应商名称不能为空", ErrInvalidParameter)
	}

	if s.isUnknownChannel(provider.Channel) {
		return fmt.Errorf("%w: 不支持的渠道类型", ErrInvalidParameter)
	}

	if provider.Endpoint == "" {
		return fmt.Errorf("%w: API入口地址不能为空", ErrInvalidParameter)
	}

	if provider.APIKey == "" {
		return fmt.Errorf("%w: API Key不能为空", ErrInvalidParameter)
	}

	if provider.APISecret == "" {
		return fmt.Errorf("%w: API Secret不能为空", ErrInvalidParameter)
	}

	if provider.Weight <= 0 {
		return fmt.Errorf("%w: 权重不能小于等于0", ErrInvalidParameter)
	}
	if provider.QPSLimit <= 0 {
		return fmt.Errorf("%w: 每秒请求数限制不能小于等于0", ErrInvalidParameter)
	}
	if provider.DailyLimit <= 0 {
		return fmt.Errorf("%w: 每日请求数限制不能小于等于0", ErrInvalidParameter)
	}

	return nil
}

func (s *providerService) isUnknownChannel(channel domain.Channel) bool {
	return channel != domain.ChannelSMS && channel != domain.ChannelEmail && channel != domain.ChannelInApp
}

// UpdateProvider 更新供应商
func (s *providerService) UpdateProvider(ctx context.Context, provider domain.Provider) error {
	if err := s.isValidateProvider(provider); err != nil {
		return err
	}
	return s.repo.Update(ctx, provider)
}

// GetProviderByID 根据ID获取供应商
func (s *providerService) GetProviderByID(ctx context.Context, id int64) (domain.Provider, error) {
	if id <= 0 {
		return domain.Provider{}, fmt.Errorf("%w: 供应商ID必须大于0", ErrInvalidParameter)
	}
	return s.repo.FindByID(ctx, id)
}

// GetProviderIDByNameAndChannel 获取供应商ID
func (s *providerService) GetProviderIDByNameAndChannel(_ context.Context, name string, channel domain.Channel) (domain.Provider, error) {
	// TODO implement me
	panic("implement me" + fmt.Sprintf("%v, %v", name, channel))
}

// GetProvidersByChannel 获取指定渠道的所有供应商
func (s *providerService) GetProvidersByChannel(ctx context.Context, channel domain.Channel) ([]domain.Provider, error) {
	if s.isUnknownChannel(channel) {
		return nil, fmt.Errorf("%w: 不支持的渠道类型", ErrInvalidParameter)
	}
	return s.repo.FindByChannel(ctx, channel)
}
