package repository

import (
	"context"
	"database/sql"

	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	daopkg "gitee.com/flycash/notification-platform/internal/service/config/internal/repository/dao"
)

type BusinessConfigRepository interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type businessConfigRepository struct {
	configDao daopkg.BusinessConfigDAO
}

// NewBusinessConfigRepository 创建业务配置仓库实例
func NewBusinessConfigRepository(configDao daopkg.BusinessConfigDAO) BusinessConfigRepository {
	return &businessConfigRepository{
		configDao: configDao,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	configMap, err := b.configDao.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	domainConfigMap := make(map[int64]domain.BusinessConfig, len(configMap))
	for id, config := range configMap {
		domainConfigMap[id] = b.toDomain(config)
	}
	return domainConfigMap, nil
}

// GetByID 根据ID获取业务配置
func (b *businessConfigRepository) GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error) {
	// 从数据库获取配置
	daoConfig, err := b.configDao.GetByID(ctx, id)
	if err != nil {
		return domain.BusinessConfig{}, err
	}

	// 将DAO对象转换为领域对象
	return b.toDomain(daoConfig), nil
}

// Delete 删除业务配置
func (b *businessConfigRepository) Delete(ctx context.Context, id int64) error {
	// 直接调用DAO层删除方法
	return b.configDao.Delete(ctx, id)
}

// SaveConfig 保存业务配置（
func (b *businessConfigRepository) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	// 将领域对象转换为DAO对象
	daoConfig := daopkg.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
	}

	// 转换JSON字段
	if config.ChannelConfig != "" {
		daoConfig.ChannelConfig = sql.NullString{
			String: config.ChannelConfig,
			Valid:  true,
		}
	}

	if config.TxnConfig != "" {
		daoConfig.TxnConfig = sql.NullString{
			String: config.TxnConfig,
			Valid:  true,
		}
	}

	if config.Quota != "" {
		daoConfig.Quota = sql.NullString{
			String: config.Quota,
			Valid:  true,
		}
	}

	if config.RetryPolicy != "" {
		daoConfig.RetryPolicy = sql.NullString{
			String: config.RetryPolicy,
			Valid:  true,
		}
	}

	// 调用DAO层保存方法
	return b.configDao.SaveConfig(ctx, daoConfig)
}

func (b *businessConfigRepository) toDomain(daoConfig daopkg.BusinessConfig) domain.BusinessConfig {
	return domain.BusinessConfig{
		ID:            daoConfig.ID,
		OwnerID:       daoConfig.OwnerID,
		OwnerType:     daoConfig.OwnerType,
		ChannelConfig: daoConfig.ChannelConfig.String,
		TxnConfig:     daoConfig.TxnConfig.String,
		RateLimit:     daoConfig.RateLimit,
		Quota:         daoConfig.Quota.String,
		RetryPolicy:   daoConfig.RetryPolicy.String,
		Ctime:         daoConfig.Ctime,
		Utime:         daoConfig.Utime,
	}
}
