package repository

import (
	"context"

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
	d                dao.CallbackLogDAO
}

func (c *callbackLogRepository) Find(ctx context.Context, startTime, batchSize, startID int64) (logs []domain.CallbackLog, nextStartID int64, err error) {
	entities, nextStartID, err := c.d.Find(ctx, startTime, batchSize, startID)
	if err != nil {
		return nil, 0, err
	}

	if int64(len(entities)) < batchSize {
		nextStartID = 0
	}

	return slice.Map(entities, func(idx int, src dao.CallbackLog) domain.CallbackLog {
		n, _ := c.notificationRepo.GetByID(context.Background(), src.NotificationID)
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
	return c.d.Update(ctx, slice.Map(logs, func(idx int, src domain.CallbackLog) dao.CallbackLog {
		return c.toDAO(src)
	}))
}

func (c *callbackLogRepository) toDAO(log domain.CallbackLog) dao.CallbackLog {
	return dao.CallbackLog{
		ID:             log.ID,
		NotificationID: log.Notification.ID,
		RetryCount:     log.RetryCount,
		NextRetryTime:  log.NextRetryTime,
		Status:         string(log.Status),
	}
}

func (c *callbackLogRepository) FindByNotificationIDs(ctx context.Context, notificationIDs []uint64) ([]domain.CallbackLog, error) {
	logs, err := c.d.FindByNotificationIDs(ctx, notificationIDs)
	if err != nil {
		return nil, err
	}
	ns, err := c.notificationRepo.BatchGetByIDs(ctx, notificationIDs)
	if err != nil {
		return nil, err
	}
	return slice.Map(logs, func(idx int, src dao.CallbackLog) domain.CallbackLog {
		return c.toDomain(src, ns[src.NotificationID])
	}), nil
}
