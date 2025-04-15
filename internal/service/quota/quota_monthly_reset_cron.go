package quota

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/repository"
	"github.com/gotomicro/ego/core/elog"
)

type MonthlyResetCron struct {
	bizRepo   repository.BusinessConfigRepository
	svc       Service
	batchSize int
	logger    *elog.Component
}

func NewQuotaMonthlyResetCron(bizRepo repository.BusinessConfigRepository, svc Service) *MonthlyResetCron {
	const batchSize = 10
	return &MonthlyResetCron{
		bizRepo:   bizRepo,
		logger:    elog.DefaultLogger,
		batchSize: batchSize,
		svc:       svc,
	}
}

func (t *MonthlyResetCron) Do(ctx context.Context) error {
	offset := 0
	for {
		const loopTimeout = time.Second * 15
		ctx, cancel := context.WithTimeout(ctx, loopTimeout)
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

func (t *MonthlyResetCron) oneLoop(ctx context.Context, offset int) (int, error) {
	const findTimeout = time.Second * 3
	ctx, cancel := context.WithTimeout(ctx, findTimeout)
	defer cancel()
	bizs, err := t.bizRepo.Find(ctx, offset, 0)
	if err != nil {
		// 一般都是无可挽回的错误了
		return 0, err
	}
	const resetTimeout = time.Second * 10
	ctx, cancel = context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	for _, cfg := range bizs {
		err = t.svc.ResetQuota(ctx, cfg)
		if err != nil {
			continue
		}
	}
	return len(bizs), nil
}
