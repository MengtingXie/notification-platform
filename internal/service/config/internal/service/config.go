package service

import (
	"context"
	"errors"

	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/repository"

	"github.com/ego-component/egorm"
)

// 错误定义
var (
	ErrInvalidParameter = errors.New("无效的参数")
	ErrConfigNotFound   = errors.New("业务配置不存在")
	ErrIDNotSet         = errors.New("业务id没有设置")
)

//go:generate mockgen -source=./config.go -destination=../../mocks/config.mock.go -package=configmocks -typed BusinessConfigService
type BusinessConfigService interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	// SaveConfig 保存非零字段
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type businessConfigService struct {
	repo repository.BusinessConfigRepository
}

// NewBusinessConfigService 创建业务配置服务实例
func NewBusinessConfigService(repo repository.BusinessConfigRepository) BusinessConfigService {
	return &businessConfigService{
		repo: repo,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigService) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	// 参数校验
	if len(ids) == 0 {
		return make(map[int64]domain.BusinessConfig), nil
	}

	// 调用仓库层方法
	return b.repo.GetByIDs(ctx, ids)
}

// GetByID 根据ID获取单个业务配置
func (b *businessConfigService) GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error) {
	// 参数校验
	if id <= 0 {
		return domain.BusinessConfig{}, ErrInvalidParameter
	}

	// 调用仓库层方法
	config, err := b.repo.GetByID(ctx, id)
	if err != nil {
		if err == egorm.ErrRecordNotFound {
			return domain.BusinessConfig{}, ErrConfigNotFound
		}
		return domain.BusinessConfig{}, err
	}

	return config, nil
}

// Delete 删除业务配置
func (b *businessConfigService) Delete(ctx context.Context, id int64) error {
	// 参数校验
	if id <= 0 {
		return ErrInvalidParameter
	}

	// 调用仓库层删除方法
	return b.repo.Delete(ctx, id)
}

// SaveNonZeroConfig 保存业务配置（仅保存非零字段）
func (b *businessConfigService) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	// 参数校验
	if config.ID <= 0 {
		return ErrIDNotSet
	}
	// 调用仓库层保存方法
	return b.repo.SaveConfig(ctx, config)
}
