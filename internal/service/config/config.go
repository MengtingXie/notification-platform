package config

import (
	"context"
	"errors"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"

	"github.com/ego-component/egorm"
)

var ErrIDNotSet = errors.New("业务id没有设置")

//go:generate mockgen -source=./config.go -destination=./mocks/config.mock.go -package=configmocks -typed BusinessConfigService
type BusinessConfigService interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	// SaveConfig 保存非零字段
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type BusinessConfigServiceV1 struct {
	repo repository.BusinessConfigRepository
}

// NewBusinessConfigService 创建业务配置服务实例
func NewBusinessConfigService(repo repository.BusinessConfigRepository) *BusinessConfigServiceV1 {
	return &BusinessConfigServiceV1{
		repo: repo,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *BusinessConfigServiceV1) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	// 参数校验
	if len(ids) == 0 {
		return make(map[int64]domain.BusinessConfig), nil
	}

	// 调用仓库层方法
	return b.repo.GetByIDs(ctx, ids)
}

// GetByID 根据ID获取单个业务配置
func (b *BusinessConfigServiceV1) GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error) {
	// 参数校验
	if id <= 0 {
		return domain.BusinessConfig{}, fmt.Errorf("%w", errs.ErrInvalidParameter)
	}

	// 调用仓库层方法
	config, err := b.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, egorm.ErrRecordNotFound) {
			return domain.BusinessConfig{}, fmt.Errorf("%w", errs.ErrConfigNotFound)
		}
		return domain.BusinessConfig{}, err
	}

	return config, nil
}

// Delete 删除业务配置
func (b *BusinessConfigServiceV1) Delete(ctx context.Context, id int64) error {
	// 参数校验
	if id <= 0 {
		return fmt.Errorf("%w", errs.ErrInvalidParameter)
	}

	// 调用仓库层删除方法
	return b.repo.Delete(ctx, id)
}

// SaveConfig 保存业务配置（仅保存非零字段）
func (b *BusinessConfigServiceV1) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	// 参数校验
	if config.ID <= 0 {
		return ErrIDNotSet
	}
	// 调用仓库层保存方法
	return b.repo.SaveConfig(ctx, config)
}
