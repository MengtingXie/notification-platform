package ratelimit

import "golang.org/x/net/context"

//go:generate mockgen -source=./types.go -package=limitmocks -destination=./mocks/limiter.mock.go Limiter
type Limiter interface {
	// Limit 判断是否应该限流
	Limit(ctx context.Context, key string) (bool, error)
	// IsLimitedAfter 检查在指定时间点后是否触发过限流
	IsLimitedAfter(ctx context.Context, key string, sinceMillis int64) (bool, error)
}
