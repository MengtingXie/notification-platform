package notification

import (
	"context"
	"time"

	clientv1 "gitee.com/flycash/notification-platform/api/proto/gen/client/v1"
	"gitee.com/flycash/notification-platform/internal/service/sender"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"github.com/ecodeclub/ekit/syncx"
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

type txNotificationService struct {
	repo      repository.TxNotificationRepository
	notiRepo  repository.NotificationRepository
	configSvc config.BusinessConfigService
	logger    *elog.Component
	lock      dlock.Client
	sender    sender.NotificationSender
}

func NewTxNotificationService(
	repo repository.TxNotificationRepository,
	notiRepo repository.NotificationRepository,
	configSvc config.BusinessConfigService,
	lock dlock.Client,
) TxNotificationService {
	return &txNotificationService{
		repo:      repo,
		configSvc: configSvc,
		logger:    elog.DefaultLogger,
		lock:      lock,
	}
}

const defaultBatchSize = 10

func (t *txNotificationService) StartTask(ctx context.Context) {
	task := &TxCheckTask{
		repo:      t.repo,
		configSvc: t.configSvc,
		logger:    t.logger,
		lock:      t.lock,
		batchSize: defaultBatchSize,
		clientMap: syncx.Map[string, clientv1.TransactionCheckServiceClient]{},
	}
	go task.Start(ctx)
}

func (t *txNotificationService) Prepare(ctx context.Context, notification domain.Notification) (uint64, error) {
	notification.Status = domain.SendStatusPrepare
	notification.SetSendTime()
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

func (t *txNotificationService) Commit(ctx context.Context, bizID int64, key string) error {
	err := t.repo.UpdateStatus(ctx, bizID, key, domain.TxNotificationStatusCommit, domain.SendStatusPending)
	if err != nil {
		return err
	}
	notification, err := t.notiRepo.GetByKey(ctx, bizID, key)
	if err != nil {
		return err
	}
	if notification.IsImmediate() {
		_, err = t.sender.Send(ctx, notification)
	}
	return err
}

func (t *txNotificationService) Cancel(ctx context.Context, bizID int64, key string) error {
	return t.repo.UpdateStatus(ctx, bizID, key, domain.TxNotificationStatusCancel, domain.SendStatusCanceled)
}
