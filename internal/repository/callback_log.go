package repository

import (
	"context"
	"log"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/slice"
)

// CallbackLogRepository 回调记录仓储接口
type CallbackLogRepository interface {
	Find(ctx context.Context, startTime, batchSize, startID int64) (logs []domain.CallbackLog, nextStartID int64, err error)
	Update(ctx context.Context, logs []domain.CallbackLog) error
	FindByNotificationIDs(ctx context.Context, notificationIDs []uint64) ([]domain.CallbackLog, error)
}

type callbackLogRepository struct {
	notificationRepo NotificationRepository
	dao              dao.CallbackLogDAO
}

func NewCallbackLogRepository(
	notificationRepo NotificationRepository,
	dao dao.CallbackLogDAO,
) CallbackLogRepository {
	return &callbackLogRepository{
		notificationRepo: notificationRepo,
		dao:              dao,
	}
}

func (c *callbackLogRepository) Find(ctx context.Context, startTime, batchSize, startID int64) (logs []domain.CallbackLog, nextStartID int64, err error) {
	log.Printf("startTime = %#v, batchsize = %#v\n", startTime, batchSize)
	entities, nextStartID, err := c.dao.Find(ctx, startTime, batchSize, startID)
	if err != nil {
		return nil, 0, err
	}

	if int64(len(entities)) < batchSize {
		nextStartID = 0
	}

	return slice.Map(entities, func(_ int, src dao.CallbackLog) domain.CallbackLog {
		n, _ := c.notificationRepo.GetByID(ctx, src.NotificationID)
		return c.toDomain(src, n)
	}), nextStartID, nil
}

func (c *callbackLogRepository) toDomain(log dao.CallbackLog, notification domain.Notification) domain.CallbackLog {
	return domain.CallbackLog{
		ID:            log.ID,
		Notification:  notification,
		RetryCount:    log.RetryCount,
		NextRetryTime: log.NextRetryTime,
		Status:        domain.CallbackLogStatus(log.Status),
	}
}

func (c *callbackLogRepository) Update(ctx context.Context, logs []domain.CallbackLog) error {
	return c.dao.Update(ctx, slice.Map(logs, func(_ int, src domain.CallbackLog) dao.CallbackLog {
		return c.toEntity(src)
	}))
}

func (c *callbackLogRepository) toEntity(log domain.CallbackLog) dao.CallbackLog {
	return dao.CallbackLog{
		ID:             log.ID,
		NotificationID: log.Notification.ID,
		RetryCount:     log.RetryCount,
		NextRetryTime:  log.NextRetryTime,
		Status:         log.Status.String(),
	}
}

func (c *callbackLogRepository) FindByNotificationIDs(ctx context.Context, notificationIDs []uint64) ([]domain.CallbackLog, error) {
	logs, err := c.dao.FindByNotificationIDs(ctx, notificationIDs)
	if err != nil {
		return nil, err
	}
	ns, err := c.notificationRepo.BatchGetByIDs(ctx, notificationIDs)
	if err != nil {
		return nil, err
	}
	return slice.Map(logs, func(_ int, src dao.CallbackLog) domain.CallbackLog {
		return c.toDomain(src, ns[src.NotificationID])
	}), nil
}
