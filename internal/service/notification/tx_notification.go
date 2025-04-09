package notification

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
)

//go:generate mockgen -source=./tx_notification.go -destination=./mocks/tx_notification.mock.go -package=notificationmocks -typed TxNotificationService
type TxNotificationService interface {
	// Prepare 准备消息,
	Prepare(ctx context.Context, notification domain.Notification) (uint64, error)
	// Commit 提交
	Commit(ctx context.Context, bizID int64, key string) error
	// Cancel 取消
	Cancel(ctx context.Context, bizID int64, key string) error
}

type TxNotificationServiceV1 struct {
	repo            repository.TxNotificationRepository
	notificationSvc Service
	configSvc       config.BusinessConfigService
	logger          *elog.Component
	lock            dlock.Client
}

func NewTxNotificationService(
	repo repository.TxNotificationRepository,
	configSvc config.BusinessConfigService,
	lock dlock.Client,
) *TxNotificationServiceV1 {
	return &TxNotificationServiceV1{
		repo:      repo,
		configSvc: configSvc,
		logger:    elog.DefaultLogger,
		lock:      lock,
	}
}

const defaultBatchSize = 10

func (t *TxNotificationServiceV1) StartTask(ctx context.Context) {
	task := &TxCheckTask{
		repo:            t.repo,
		notificationSvc: t.notificationSvc,
		configSvc:       t.configSvc,
		logger:          t.logger,
		lock:            t.lock,
		batchSize:       defaultBatchSize,
		clientMap:       syncx.Map[string, *egrpc.Component]{},
	}
	go task.Start(ctx)
}

func (t *TxNotificationServiceV1) Prepare(ctx context.Context, notification domain.Notification) (uint64, error) {
	notification.Status = domain.SendStatusPrepare
	txn := domain.TxNotification{
		Notification: notification,
		Key:          notification.Key,
		BizID:        notification.BizID,
		Status:       domain.TxNotificationStatusPrepare,
	}

	cfg, err := t.configSvc.GetByID(ctx, notification.BizID)
	if err == nil {
		now := time.Now().UnixMilli()
		const second = 1000
		txn.NextCheckTime = now + int64(cfg.TxnConfig.InitialDelay*second)
	}

	return t.repo.Create(ctx, txn)
}

func (t *TxNotificationServiceV1) Commit(ctx context.Context, bizID int64, key string) error {
	return t.repo.UpdateStatus(ctx, bizID, key, domain.TxNotificationStatusCommit.String(), string(domain.SendStatusPending))
}

func (t *TxNotificationServiceV1) Cancel(ctx context.Context, bizID int64, key string) error {
	return t.repo.UpdateStatus(ctx, bizID, key, domain.TxNotificationStatusCancel.String(), string(domain.SendStatusCanceled))
}
