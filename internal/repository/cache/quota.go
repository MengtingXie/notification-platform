package cache

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
)

type IncrItem struct {
	BizID   int64
	Channel domain.Channel
	Val     int32
}

type QuotaCache interface {
	CreateOrUpdate(ctx context.Context, quota ...domain.Quota) error
	Find(ctx context.Context, bizID int64, channel domain.Channel) (domain.Quota, error)
	Incr(ctx context.Context, bizID int64, channel domain.Channel, quota int32) error
	Decr(ctx context.Context, bizID int64, channel domain.Channel, quota int32) error
	MutiIncr(ctx context.Context, items []IncrItem) error
	MutiDecr(ctx context.Context, items []IncrItem) error
}
