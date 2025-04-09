package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"gitee.com/flycash/notification-platform/internal/repository/cache"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
)

type BusinessConfigRepository interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type businessConfigRepository struct {
	dao dao.BusinessConfigDAO
	localCache cache.ConfigCache
	redisCache cache.ConfigCache
}

// NewBusinessConfigRepository 创建业务配置仓库实例
func NewBusinessConfigRepository(configDao dao.BusinessConfigDAO) BusinessConfigRepository {
	return &businessConfigRepository{
		dao: configDao,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	configMap, err := b.dao.GetByIDs(ctx, ids)
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
	c, err := b.dao.GetByID(ctx, id)
	if err != nil {
		return domain.BusinessConfig{}, err
	}
	// 将DAO对象转换为领域对象
	return b.toDomain(c), nil
}

// Delete 删除业务配置
func (b *businessConfigRepository) Delete(ctx context.Context, id int64) error {
	// 直接调用DAO层删除方法
	return b.dao.Delete(ctx, id)
}

// SaveConfig 保存业务配置（
func (b *businessConfigRepository) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	return b.dao.SaveConfig(ctx, b.toEntity(config))
}

func (b *businessConfigRepository) toDomain(daoConfig dao.BusinessConfig) domain.BusinessConfig {
	domainCfg := domain.BusinessConfig{
		ID:        daoConfig.ID,
		OwnerID:   daoConfig.OwnerID,
		OwnerType: daoConfig.OwnerType,
		RateLimit: daoConfig.RateLimit,
		Ctime:     daoConfig.Ctime,
		Utime:     daoConfig.Utime,
	}
	if daoConfig.ChannelConfig.Valid {
		domainCfg.ChannelConfig = unmarsal[domain.ChannelConfig](daoConfig.ChannelConfig.String)
	}
	if daoConfig.TxnConfig.Valid {
		domainCfg.TxnConfig = unmarsal[domain.TxnConfig](daoConfig.TxnConfig.String)
	}
	if daoConfig.Quota.Valid {
		domainCfg.Quota = unmarsal[domain.QuotaConfig](daoConfig.Quota.String)
	}
	if daoConfig.RetryPolicy.Valid {
		domainCfg.RetryPolicy = unmarsal[domain.RetryConfig](daoConfig.RetryPolicy.String)
	}
	return domainCfg
}

func (b *businessConfigRepository) toEntity(config domain.BusinessConfig) dao.BusinessConfig {
	daoConfig := dao.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
	}
	if config.ChannelConfig != nil {
		daoConfig.ChannelConfig = marshal(config.ChannelConfig)
	}
	if config.RetryPolicy != nil {
		daoConfig.RetryPolicy = marshal(config.RetryPolicy)
	}
	if config.TxnConfig != nil {
		daoConfig.TxnConfig = marshal(config.TxnConfig)
	}
	if config.Quota != nil {
		daoConfig.Quota = marshal(config.Quota)
	}
	return daoConfig
}

func marshal(v any) sql.NullString {
	byteV, _ := json.Marshal(v)
	return sql.NullString{
		String: string(byteV),
		Valid:  true,
	}
}

func unmarsal[T any](v string) *T {
	var t T
	_ = json.Unmarshal([]byte(v), &t)
	return &t
}
