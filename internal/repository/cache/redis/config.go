package redis

import (
	"context"
	"encoding/json"

	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var ErrorKeyNotFound = errors.New("config not found in redis")

type Cache struct {
	rdb *redis.Client
}


func (c *Cache) Get(ctx context.Context, bizID int64) (domain.BusinessConfig, error) {
	key := cache.ConfigKey(bizID)
	// 从Redis获取数据
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 键不存在
			return domain.BusinessConfig{}, ErrorKeyNotFound
		}
		// 其他错误
		return domain.BusinessConfig{}, fmt.Errorf("failed to get config from redis %w", err)
	}

	// 反序列化数据
	var cfg domain.BusinessConfig
	err = json.Unmarshal([]byte(val), &cfg)
	if err != nil {
		return domain.BusinessConfig{}, fmt.Errorf("failed to unmarshal config data %w", err)
	}

	return cfg, nil
}

func (c *Cache) Set(ctx context.Context, cfg domain.BusinessConfig) error {
	key := cache.ConfigKey(cfg.ID)

	// 序列化数据
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config data %w", err)
	}

	// 存储到Redis
	err = c.rdb.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set config from redis %w", err)
	}
	return nil
}
