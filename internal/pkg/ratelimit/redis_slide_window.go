package ratelimit

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

var (
	//go:embed lua/slide_window.lua
	slidingWindowScript string

	//go:embed lua/check_limited_event.lua
	checkLimitedEventScript string

	_ Limiter = (*RedisSlidingWindowLimiter)(nil)
)

type RedisSlidingWindowLimiter struct {
	cmd       redis.Cmdable
	interval  time.Duration
	rate      int
	keyPrefix string
}

// NewRedisSlidingWindowLimiter 创建一个基于Redis的滑动窗口限流器
func NewRedisSlidingWindowLimiter(cmd redis.Cmdable, interval time.Duration, rate int) *RedisSlidingWindowLimiter {
	return &RedisSlidingWindowLimiter{
		cmd:       cmd,
		interval:  interval,
		rate:      rate,
		keyPrefix: "ratelimit:",
	}
}

// Limit 判断是否应该限流
func (r *RedisSlidingWindowLimiter) Limit(ctx context.Context, key string) (bool, error) {
	return r.cmd.Eval(ctx, slidingWindowScript,
		[]string{r.getCountKey(key), r.getLimitedEventsKey(key)},
		r.interval.Milliseconds(),
		r.rate,
		time.Now().UnixMilli(),
	).Bool()
}

// getCountKey 获取请求计数的Redis键
func (r *RedisSlidingWindowLimiter) getCountKey(key string) string {
	return fmt.Sprintf("%scount:%s", r.keyPrefix, key)
}

// getLimitedEventsKey 获取限流事件记录的Redis键
func (r *RedisSlidingWindowLimiter) getLimitedEventsKey(key string) string {
	return fmt.Sprintf("%slimitedEvents:%s", r.keyPrefix, key)
}

// IsLimitedAfter 检查在指定时间点后是否触发过限流
func (r *RedisSlidingWindowLimiter) IsLimitedAfter(ctx context.Context, key string, sinceMillis int64) (bool, error) {
	return r.cmd.Eval(ctx, checkLimitedEventScript,
		[]string{r.getLimitedEventsKey(key)},
		sinceMillis,
		time.Now().UnixMilli()).Bool()
}
