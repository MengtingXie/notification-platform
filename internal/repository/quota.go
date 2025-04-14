package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/slice"
)

type QuotaRepository interface {
	CreateOrUpdate(ctx context.Context, quota ...domain.Quota) error
	Find(ctx context.Context, bizID int64, channel domain.Channel) (domain.Quota, error)
}

type quotaRepository struct {
	dao dao.QuotaDAO
}

func NewQuotaRepository(dao dao.QuotaDAO) QuotaRepository {
	return &quotaRepository{dao: dao}
}

func (q *quotaRepository) CreateOrUpdate(ctx context.Context, quota ...domain.Quota) error {
	qs := slice.Map(quota, func(_ int, src domain.Quota) dao.Quota {
		return dao.Quota{
			Quota:   src.Quota,
			BizID:   src.BizID,
			Channel: src.Channel.String(),
		}
	})
	return q.dao.CreateOrUpdate(ctx, qs...)
}

func (q *quotaRepository) Find(ctx context.Context, bizID int64, channel domain.Channel) (domain.Quota, error) {
	found, err := q.dao.Find(ctx, bizID, channel.String())
	if err != nil {
		return domain.Quota{}, err
	}
	return domain.Quota{
		BizID:   found.BizID,
		Quota:   found.Quota,
		Channel: domain.Channel(found.Channel),
	}, nil
}
