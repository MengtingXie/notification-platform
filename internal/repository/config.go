package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"gitee.com/flycash/notification-platform/internal/repository/cache/local"
	"gitee.com/flycash/notification-platform/internal/repository/cache/redis"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/sqlx"
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
	redisCache *redis.Cache,
) BusinessConfigRepository {
	return &businessConfigRepository{
		dao:        configDao,
		localCache: localCache,
		redisCache: redisCache,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	// 1. 先从本地缓存批量获取
	result, err := b.localCache.GetConfigs(ctx, ids)
	if err != nil {
		b.logger.Error("从本地缓存批量获取失败", elog.Any("err", err))
		// 如果本地缓存出错，初始化空结果集继续处理
		result = make(map[int64]domain.BusinessConfig)
	}

	// 找出本地缓存中未命中的IDs
	var missedIDs []int64
	for _, id := range ids {
		if _, ok := result[id]; !ok {
			missedIDs = append(missedIDs, id)
		}
	}

	// 如果所有数据都在本地缓存中找到，直接返回
	if len(missedIDs) == 0 {
		return result, nil
	}

	// 2. 从Redis缓存批量获取本地缓存中未找到的配置
	redisConfigs, err := b.redisCache.GetConfigs(ctx, missedIDs)
	if err != nil {
		b.logger.Error("从Redis缓存批量获取失败", elog.Any("err", err))
		// 即使Redis出错，我们也继续处理
	} else {
		// 将Redis中找到的配置添加到结果集
		var configsToLocalCache []domain.BusinessConfig
		for id, config := range redisConfigs {
			result[id] = config
			configsToLocalCache = append(configsToLocalCache, config)
		}

		// 批量更新本地缓存
		if len(configsToLocalCache) > 0 {
			lerr := b.localCache.SetConfigs(ctx, configsToLocalCache)
			if lerr != nil {
				b.logger.Error("批量更新本地缓存失败", elog.Any("err", lerr))
			}
		}
	}

	// 找出Redis缓存中也未命中的IDs
	var notFoundIDs []int64
	for _, id := range missedIDs {
		if _, ok := redisConfigs[id]; !ok {
			notFoundIDs = append(notFoundIDs, id)
		}
	}

	// 如果所有数据都在本地缓存或Redis中找到，直接返回
	if len(notFoundIDs) == 0 {
		return result, nil
	}

	// 3. 从数据库获取缓存中未找到的配置
	configMap, err := b.dao.GetByIDs(ctx, notFoundIDs)
	if err != nil {
		return nil, err
	}

	// 4. 处理从数据库获取的结果，批量更新缓存并添加到结果集

	configsToCache := make([]domain.BusinessConfig, 0, len(ids))
	for id := range configMap {
		config := configMap[id]
		domainConfig := b.toDomain(config)
		result[id] = domainConfig
		configsToCache = append(configsToCache, domainConfig)
	}

	// 批量更新缓存
	if len(configsToCache) > 0 {
		// 更新本地缓存
		lerr := b.localCache.SetConfigs(ctx, configsToCache)
		if lerr != nil {
			b.logger.Error("批量更新本地缓存失败", elog.Any("err", lerr))
		}

		// 更新Redis缓存
		rerr := b.redisCache.SetConfigs(ctx, configsToCache)
		if rerr != nil {
			b.logger.Error("批量更新Redis缓存失败", elog.Any("err", rerr))
		}
	}

	return result, nil
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
	cfg, err := b.dao.SaveConfig(ctx, b.toEntity(config))
	if err != nil {
		return err
	}
	rerr := b.redisCache.Set(ctx, b.toDomain(cfg))
	if rerr != nil {
		b.logger.Error("更新redis缓存失败", elog.Any("err", rerr), elog.Int("bizId", int(config.ID)))
	}
	return nil
}

func (b *businessConfigRepository) toDomain(config dao.BusinessConfig) domain.BusinessConfig {
	domainCfg := domain.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
	}
	if config.ChannelConfig.Valid {
		domainCfg.ChannelConfig = &config.ChannelConfig.Val
	}
	if config.TxnConfig.Valid {
		domainCfg.TxnConfig = &config.TxnConfig.Val
	}
	if config.Quota.Valid {
		domainCfg.Quota = &config.Quota.Val
	}
	if config.CallbackConfig.Valid {
		domainCfg.CallbackConfig = &config.CallbackConfig.Val
	}
	return domainCfg
}

func (b *businessConfigRepository) toEntity(config domain.BusinessConfig) dao.BusinessConfig {
	businessConfig := dao.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
	}

	if config.ChannelConfig != nil {
		businessConfig.ChannelConfig = sqlx.JsonColumn[domain.ChannelConfig]{
			Val:   *config.ChannelConfig,
			Valid: true,
		}
	}

	if config.TxnConfig != nil {
		businessConfig.TxnConfig = sqlx.JsonColumn[domain.TxnConfig]{
			Val:   *config.TxnConfig,
			Valid: true,
		}
	}

	if config.Quota != nil {
		businessConfig.Quota = sqlx.JsonColumn[domain.QuotaConfig]{
			Val:   *config.Quota,
			Valid: true,
		}
	}

	if config.CallbackConfig != nil {
		businessConfig.CallbackConfig = sqlx.JsonColumn[domain.CallbackConfig]{
			Val:   *config.CallbackConfig,
			Valid: true,
		}
	}

	return businessConfig
}
