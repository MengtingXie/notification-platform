package local

import (
	"context"
	"encoding/json"
	"github.com/gotomicro/ego/core/elog"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	ca "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var ErrorKeyNotFound = errors.New("key not found")

const defaultTimeout = 3 * time.Second

type Cache struct {
	rdb    *redis.Client
	logger *elog.Component
	c      ca.Cache
}

func (l *Cache) Get(ctx context.Context, bizID int64) (domain.BusinessConfig, error) {
	key := cache.ConfigKey(bizID)
	v, ok := l.c.Get(key)
	if !ok {
		return domain.BusinessConfig{}, ErrorKeyNotFound
	}
	return v.(domain.BusinessConfig), nil
}

func (l *Cache) Set(ctx context.Context, cfg domain.BusinessConfig) error {
	key := cache.ConfigKey(cfg.ID)
	l.c.Set(key, cfg, -1)
	return nil
}


func (l *Cache) NewLocalCache(rdb *redis.Client, c ca.Cache)*Cache {
	return &Cache{
		rdb:    rdb,
		logger: elog.DefaultLogger,
		c:      c,
	}
}

// 监控redis里的东西
func (l *Cache) loop(ctx context.Context) {
	pubsub := l.rdb.PSubscribe(ctx, "__keyspace@*__:config:*")
	defer pubsub.Close()
	// 开始监听消息
	ch := pubsub.Channel()
	for msg := range ch {
		channel := msg.Channel
		key := msg.Payload
		channelStrList := strings.Split(channel, ":")
		if len(channelStrList) < 2 {
			l.logger.Error("监听redis键不正确", elog.String("channel", channel))
			continue
		}
		eventType := strings.Split(channel, ":")[1]
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
		l.c.Set(key, config, -1)
	case "del":
		l.c.Delete(key)
	}
}
