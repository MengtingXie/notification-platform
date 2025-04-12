package retry

import (
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/retry/strategy"
)

// NewRetry 当配置不对的时候，会返回默认的重试策略
func NewRetry(cfg Config) (strategy.Strategy, error) {
	// 根据 config 中的字段来检测
	switch cfg.Type {
	case "fixed":
		return strategy.NewFixedIntervalRetryStrategy(cfg.FixedInterval.Interval, cfg.FixedInterval.MaxRetries), nil
	case "exponential":
		return strategy.NewExponentialBackoffRetryStrategy(cfg.ExponentialBackoff.InitialInterval, cfg.ExponentialBackoff.MaxInterval, cfg.ExponentialBackoff.MaxRetries), nil
	default:
		return nil, fmt.Errorf("unknown retry type: %s", cfg.Type)
	}
}

type Config struct {
	Type               string                    `json:"type"`
	FixedInterval      *FixedIntervalConfig      `json:"fixedInterval"`
	ExponentialBackoff *ExponentialBackoffConfig `json:"exponentialBackoff"`
}

type ExponentialBackoffConfig struct {
	// 初始重试间隔 单位ms
	InitialInterval time.Duration `json:"initialInterval"`
	MaxInterval     time.Duration `json:"maxInterval"`
	// 最大重试次数
	MaxRetries int32 `json:"maxRetries"`
}

type FixedIntervalConfig struct {
	MaxRetries int32         `json:"maxRetries"`
	Interval   time.Duration `json:"interval"`
}
