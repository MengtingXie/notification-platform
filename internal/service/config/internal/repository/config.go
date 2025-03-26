package repository

import (
	"context"
	"database/sql"
	"errors"

	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/repository/dao"

	"github.com/ego-component/egorm"
)

type BusinessConfigRepository interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	// SaveConfig 保存非零字段
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type businessConfigRepository struct {
	dao dao.BusinessConfigDAO
}

// NewBusinessConfigRepository 创建业务配置仓库实例
func NewBusinessConfigRepository(configDao dao.BusinessConfigDAO) BusinessConfigRepository {
	return &businessConfigRepository{
		dao: configDao,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	result := make(map[int64]domain.BusinessConfig)

	// 循环查询每个ID的配置（实际使用中可以优化为批量查询）
	for _, id := range ids {
		config, err := b.GetByID(ctx, id)
		if err != nil {
			// 如果是未找到记录的错误，则跳过
			if errors.Is(err, egorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		result[id] = config
	}

	return result, nil
}

// GetByID 根据ID获取业务配置
func (b *businessConfigRepository) GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error) {
	// 从数据库获取配置
	c, err := b.dao.GetByID(ctx, id)
	if err != nil {
		return domain.BusinessConfig{}, err
	}
	// 将DAO对象转换为领域对象
	return b.toDomain(c), nil
}

func (b *businessConfigRepository) toDomain(c dao.BusinessConfig) domain.BusinessConfig {
	return domain.BusinessConfig{
		ID:            c.ID,
		OwnerID:       c.OwnerID,
		OwnerType:     c.OwnerType,
		ChannelConfig: c.ChannelConfig.String,
		TxnConfig:     c.TxnConfig.String,
		RateLimit:     c.RateLimit,
		Quota:         c.Quota.String,
		RetryPolicy:   c.RetryPolicy.String,
		Ctime:         c.Ctime,
		Utime:         c.Utime,
	}
}

// Delete 删除业务配置
func (b *businessConfigRepository) Delete(ctx context.Context, id int64) error {
	// 直接调用DAO层删除方法
	return b.dao.Delete(ctx, id)
}

// SaveConfig 保存业务配置（仅保存非零字段）
func (b *businessConfigRepository) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	// 将领域对象转换为DAO对象
	daoConfig := dao.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
		ChannelConfig: sql.NullString{
			String: config.ChannelConfig,
			Valid:  config.ChannelConfig != "",
		},
		TxnConfig: sql.NullString{
			String: config.TxnConfig,
			Valid:  config.TxnConfig != "",
		},
		Quota: sql.NullString{
			String: config.Quota,
			Valid:  config.Quota != "",
		},
		RetryPolicy: sql.NullString{
			String: config.RetryPolicy,
			Valid:  config.Quota != "",
		},
	}
	// 调用DAO层保存方法
	return b.dao.SaveConfig(ctx, daoConfig)
}
