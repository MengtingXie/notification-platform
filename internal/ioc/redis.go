package ioc

import (
	redishook "gitee.com/flycash/notification-platform/internal/pkg/redis"
	"github.com/gotomicro/ego/core/econf"
	"github.com/redis/go-redis/v9"
)

func InitRedisClient() *redis.Client {
	type Config struct {
		Addr string
	}
	var cfg Config
	err := econf.UnmarshalKey("redis", &cfg)
	if err != nil {
		panic(err)
	}
	cmd := redis.NewClient(&redis.Options{
		Addr: cfg.Addr,
	})
	cmd = redishook.WithTracing(cmd)
	return cmd
}
