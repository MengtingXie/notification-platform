package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/slice"
)

type QuotaRepository interface {
	CreateQuotaOrUpdate(ctx context.Context, quota ...domain.Quota) error
}

type quotaRepository struct {
	dao dao.QuotaDAO
}

func NewQuotaRepository(dao dao.QuotaDAO) QuotaRepository {
	return &quotaRepository{dao: dao}
}

func (repo *quotaRepository) CreateQuotaOrUpdate(ctx context.Context, quota ...domain.Quota) error {
	qs := slice.Map(quota, func(_ int, src domain.Quota) dao.Quota {
		return dao.Quota{
			Quota:   src.Quota,
			BizID:   src.BizID,
			Channel: src.Channel.String(),
		}
	})
	return repo.dao.CreateQuotaOrUpdate(ctx, qs...)
}
