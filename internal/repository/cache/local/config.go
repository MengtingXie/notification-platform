package local

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gotomicro/ego/core/elog"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	ca "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTimeout = 3 * time.Second
)

type Cache struct {
	rdb    *redis.Client
	logger *elog.Component
	c      *ca.Cache
}

func (l *Cache) GetConfigs(ctx context.Context, bizIDs []int64) (map[int64]domain.BusinessConfig, error) {
	configMap := make(map[int64]domain.BusinessConfig)
	for _, bizID := range bizIDs {
		v, ok := l.c.Get(cache.ConfigKey(bizID))
		if ok {
			configMap[bizID] = v.(domain.BusinessConfig)
		}
	}
	return configMap, nil
}

func (l *Cache) SetConfigs(ctx context.Context, configs []domain.BusinessConfig) error {
	for _, config := range configs {
		l.c.Set(cache.ConfigKey(config.ID), config, cache.DefaultExpiredTime)
	}
	return nil
}

func (l *Cache) Del(ctx context.Context, bizID int64) error {
	l.c.Delete(cache.ConfigKey(bizID))
	return nil
}

func (l *Cache) Get(ctx context.Context, bizID int64) (domain.BusinessConfig, error) {
	key := cache.ConfigKey(bizID)
	v, ok := l.c.Get(key)
	if !ok {
		return domain.BusinessConfig{}, cache.ErrorKeyNotFound
	}
	return v.(domain.BusinessConfig), nil
}

func (l *Cache) Set(ctx context.Context, cfg domain.BusinessConfig) error {
	key := cache.ConfigKey(cfg.ID)
	l.c.Set(key, cfg, cache.DefaultExpiredTime)
	return nil
}

func NewLocalCache(rdb *redis.Client, c *ca.Cache) *Cache {
	localCache := &Cache{
		rdb:    rdb,
		logger: elog.DefaultLogger,
		c:      c,
	}
	// 开启监控redis里的内容
	go localCache.loop(context.Background())
	return localCache
}

// 监控redis里的东西
func (l *Cache) loop(ctx context.Context) {
	pubsub := l.rdb.PSubscribe(ctx, "__keyspace@*__:config:*")
	defer pubsub.Close()
	// 开始监听消息
	ch := pubsub.Channel()
	for msg := range ch {

		channel := msg.Channel
		eventType := msg.Payload
		l.logger.Info("监控到redis更新消息", elog.String("key", msg.Channel), elog.String("payload", string(msg.Payload)))
		channelStrList := strings.SplitN(channel, ":", 2)
		if len(channelStrList) < 2 {
			l.logger.Error("监听redis键不正确", elog.String("channel", channel))
			continue
		}
		key := channelStrList[1]
		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		l.handleConfigChange(ctx, key, eventType)
		cancel()
	}
}

func (l *Cache) handleConfigChange(ctx context.Context, key string, event string) {
	// 自定义业务逻辑（如动态更新配置）
	switch event {
	case "set":
		res := l.rdb.Get(ctx, key)
		if res.Err() != nil {
			l.logger.Error("订阅完获取键失败", elog.String("key", key))
		}
		var config domain.BusinessConfig
		err := json.Unmarshal([]byte(res.Val()), &config)
		if err != nil {
			l.logger.Error("序列化失败", elog.String("key", key), elog.String("val", res.Val()))
		}
		l.c.Set(key, config, cache.DefaultExpiredTime)
	case "del":
		l.c.Delete(key)
	}
}
