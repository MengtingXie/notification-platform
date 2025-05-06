package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
)

type quotaRepositoryV2 struct {
	ca cache.QuotaCache
}

func NewQuotaRepositoryV2(ca cache.QuotaCache) QuotaRepository {
	return &quotaRepositoryV2{ca}
}

func (q *quotaRepositoryV2) CreateOrUpdate(ctx context.Context, quota ...domain.Quota) error {
	return q.ca.CreateOrUpdate(ctx, quota...)
}

func (q *quotaRepositoryV2) Find(ctx context.Context, bizID int64, channel domain.Channel) (domain.Quota, error) {
	return q.ca.Find(ctx, bizID, channel)
}
