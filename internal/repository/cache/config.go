package cache

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
)

const (
	ConfigPrefix = "config"
)

type ConfigCache interface {
	Get(ctx context.Context, bizID int64) (domain.BusinessConfig, error)
	Set(ctx context.Context, cfg domain.BusinessConfig) error
}

func ConfigKey(bizID int64) string {
	return fmt.Sprintf("%s:%d", ConfigPrefix, bizID)
}
