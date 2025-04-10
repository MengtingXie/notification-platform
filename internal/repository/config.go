package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"gitee.com/flycash/notification-platform/internal/repository/cache/local"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/gotomicro/ego/core/elog"
)

type BusinessConfigRepository interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	SaveConfig(ctx context.Context, config domain.BusinessConfig) error
}

type businessConfigRepository struct {
	dao        dao.BusinessConfigDAO
	localCache cache.ConfigCache
	redisCache cache.ConfigCache
	logger     *elog.Component
}

// NewBusinessConfigRepository 创建业务配置仓库实例
func NewBusinessConfigRepository(
	configDao dao.BusinessConfigDAO,
	localCache *local.Cache,
	redisCache *redis.Cache) BusinessConfigRepository {
	return &businessConfigRepository{
		dao:        configDao,
		localCache: localCache,
		redisCache: redisCache,
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

	cfg, localErr := b.localCache.Get(ctx, id)
	if localErr == nil {
		return cfg, nil
	}
	cfg, redisErr := b.redisCache.Get(ctx, id)
	if redisErr == nil {
		// 刷新本地缓存
		lerr := b.localCache.Set(ctx, cfg)
		if lerr != nil {
			b.logger.Error("刷新本地缓存失败", elog.Any("err", lerr), elog.Int("bizId", int(id)))
		}
		return cfg, nil
	}

	c, err := b.dao.GetByID(ctx, id)
	if err != nil {
		return domain.BusinessConfig{}, err
	}
	domainConfig := b.toDomain(c)
	// 刷新本地缓存+redis
	lerr := b.localCache.Set(ctx, domainConfig)
	if lerr != nil {
		b.logger.Error("刷新本地缓存失败", elog.Any("err", lerr), elog.Int("bizId", int(id)))
	}
	rerr := b.redisCache.Set(ctx, domainConfig)
	if rerr != nil {
		b.logger.Error("刷新redis缓存失败", elog.Any("err", rerr), elog.Int("bizId", int(id)))
	}
	// 将DAO对象转换为领域对象
	return domainConfig, nil
}

// Delete 删除业务配置
func (b *businessConfigRepository) Delete(ctx context.Context, id int64) error {
	err := b.dao.Delete(ctx, id)
	if err != nil {
		return err
	}
	rerr := b.redisCache.Del(ctx, id)
	if rerr != nil {
		b.logger.Error("删除redis缓存失败", elog.Any("err", rerr), elog.Int("bizId", int(id)))
	}
	return nil
}

// SaveConfig 保存业务配置
func (b *businessConfigRepository) SaveConfig(ctx context.Context, config domain.BusinessConfig) error {
	cfg,err := b.dao.SaveConfig(ctx, b.toEntity(config))
	if err != nil {
		return err
	}
	rerr := b.redisCache.Set(ctx, b.toDomain(cfg))
	if rerr != nil {
		b.logger.Error("更新redis缓存失败", elog.Any("err", rerr), elog.Int("bizId", int(config.ID)))
	}
	return nil
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
