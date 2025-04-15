package local

import (
	"context"
	"encoding/json"
	"errors"
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

func (l *Cache) GetConfigs(_ context.Context, bizIDs []int64) (map[int64]domain.BusinessConfig, error) {
	configMap := make(map[int64]domain.BusinessConfig)
	for _, bizID := range bizIDs {
		v, ok := l.c.Get(cache.ConfigKey(bizID))
		if ok {
			vv, ok := v.(domain.BusinessConfig)
			if !ok {
				return configMap, errors.New("数据类型不正确")
			}
			configMap[bizID] = vv
		}
	}
	return configMap, nil
}

func (l *Cache) SetConfigs(_ context.Context, configs []domain.BusinessConfig) error {
	for _, config := range configs {
		l.c.Set(cache.ConfigKey(config.ID), config, cache.DefaultExpiredTime)
	}
	return nil
}

func (l *Cache) Del(_ context.Context, bizID int64) error {
	l.c.Delete(cache.ConfigKey(bizID))
	return nil
}

func (l *Cache) Get(_ context.Context, bizID int64) (domain.BusinessConfig, error) {
	key := cache.ConfigKey(bizID)
	v, ok := l.c.Get(key)
	if !ok {
		return domain.BusinessConfig{}, cache.ErrKeyNotFound
	}
	vv, ok := v.(domain.BusinessConfig)
	if !ok {
		return domain.BusinessConfig{}, errors.New("数据类型不正确")
	}
	return vv, nil
}

func (l *Cache) Set(_ context.Context, cfg domain.BusinessConfig) error {
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
	// 我要在这里监听 Redis 的 key 变更，更新本地缓存
	go localCache.loop(context.Background())
	return localCache
}

// 监控redis里的东西
func (l *Cache) loop(ctx context.Context) {
	// 就这个 channel 的表达式，你去问 deepseek
	pubsub := l.rdb.PSubscribe(ctx, "__keyspace@*__:config:*")
	defer pubsub.Close()
	ch := pubsub.Channel()
	for msg := range ch {
		// 在线上环境，小心别把敏感数据打出来了
		// 比如说你的 channel 里面包含了手机号码，你就别打了
		l.logger.Info("监控到 Redis 更新消息",
			elog.String("key", msg.Channel), elog.String("payload", msg.Payload))
		const channelMinLen = 2
		channel := msg.Channel
		channelStrList := strings.SplitN(channel, ":", channelMinLen)
		if len(channelStrList) < 2 {
			l.logger.Error("监听到非法 Redis key", elog.String("channel", msg.Channel))
			continue
		}
		// config:133 => 133
		key := channelStrList[1]
		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		eventType := msg.Payload
		l.handleConfigChange(ctx, key, eventType)
		cancel()
	}
}

func (l *Cache) handleConfigChange(ctx context.Context, key, event string) {
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
