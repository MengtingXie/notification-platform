package quota

import (
	"context"
	"gitee.com/flycash/notification-platform/internal/repository"
	"github.com/gotomicro/ego/core/elog"
	"time"
)

type QuotaMonthlyResetCron struct {
	bizRepo   repository.BusinessConfigRepository
	svc       Service
	batchSize int
	logger    *elog.Component
}

func NewQuotaMonthlyResetCron(bizRepo repository.BusinessConfigRepository, svc Service) *QuotaMonthlyResetCron {
	return &QuotaMonthlyResetCron{
		bizRepo:   bizRepo,
		logger:    elog.DefaultLogger,
		batchSize: 10,
		svc:       svc}
}

func (t *QuotaMonthlyResetCron) Do(ctx context.Context) error {
	offset := 0
	for {
		ctx, cancel := context.WithTimeout(ctx, time.Second*15)
		cnt, err := t.oneLoop(ctx, offset)
		cancel()
		if err != nil {
			t.logger.Error("查找 Biz配置失败", elog.FieldErr(err))
			// 继续尝试下一批
			offset += t.batchSize
			continue
		}

		if cnt < t.batchSize {
			return nil
		}
		offset += cnt
	}
}

func (t *QuotaMonthlyResetCron) oneLoop(ctx context.Context, offset int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	bizs, err := t.bizRepo.Find(ctx, offset, 0)
	if err != nil {
		// 一般都是无可挽回的错误了
		return 0, err
	}

	ctx, cancel = context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	for _, cfg := range bizs {
		err = t.svc.ResetQuota(ctx, cfg)
		if err != nil {

			continue
		}
	}
	return len(bizs), nil
}
