package notification

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/errs"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification/retry"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
)

var ErrUpdateStatusFailed = errors.New("update status failed")

//go:generate mockgen -source=./tx_notification.go -destination=./mocks/tx_notification.mock.go -package=notificationmocks -typed TxNotificationService
type TxNotificationService interface {
	// Prepare 准备消息,
	Prepare(ctx context.Context, txNotification domain.TxNotification) (uint64, error)
	// Commit 提交
	Commit(ctx context.Context, bizID int64, key string) error
	// Cancel 取消
	Cancel(ctx context.Context, bizID int64, key string) error
	// GetNotification 获取事务
	GetNotification(ctx context.Context, bizID int64, key string) (domain.TxNotification, error)
}

type TxNotificationServiceV1 struct {
	repo            repository.TxNotificationRepository
	notificationSvc Service
	configSvc       config.BusinessConfigService
	// retryStrategyBuilder retry.Builder
	logger *elog.Component
	lock   dlock.Client
}

func NewTxNotificationService(
	repo repository.TxNotificationRepository,
	notificationSvc Service,
	configSvc config.BusinessConfigService,
	retryStrategyBuilder retry.Builder,
	lock dlock.Client,
) *TxNotificationServiceV1 {
	return &TxNotificationServiceV1{
		repo:            repo,
		notificationSvc: notificationSvc,
		configSvc:       configSvc,
		// retryStrategyBuilder: retryStrategyBuilder,
		logger: elog.DefaultLogger,
		lock:   lock,
	}
}

const defaultBatchSize = 10

func (t *TxNotificationServiceV1) StartTask(ctx context.Context) {
	task := &TxCheckTask{
		repo:            t.repo,
		notificationSvc: t.notificationSvc,
		configSvc:       t.configSvc,
		// retryStrategyBuilder: t.retryStrategyBuilder,
		logger:    t.logger,
		lock:      t.lock,
		batchSize: defaultBatchSize,
		clientMap: syncx.Map[string, *egrpc.Component]{},
	}
	go task.Start(ctx)
}

func (t *TxNotificationServiceV1) GetNotification(ctx context.Context, bizID int64, key string) (domain.TxNotification, error) {
	txn, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return domain.TxNotification{}, err
	}
	n, err := t.notificationSvc.GetByID(ctx, txn.Notification.ID)
	if err != nil {
		return domain.TxNotification{}, err
	}
	txn.Notification = n
	return txn, nil
}

func (t *TxNotificationServiceV1) Prepare(ctx context.Context, txNotification domain.TxNotification) (uint64, error) {
	txNotification.SetSendTime()
	noti, nerr := t.notificationSvc.Create(ctx, txNotification.Notification)
	if nerr != nil {
		if errors.Is(nerr, errs.ErrNotificationDuplicate) {
			txn, err := t.repo.GetByBizIDKey(ctx, txNotification.BizID, txNotification.Key)
			if err != nil {
				return 0, err
			}
			return txn.Notification.ID, nil
		}
		return 0, nerr
	}
	txNotification.Notification = noti
	//conf, err := t.configSvc.GetByID(ctx, txNotification.BizID)
	//if err == nil {
	// 找到配置
	// 这一块，
	//txNotification.NextCheckTime = int64(time.Minute * 30)
	//}
	_, err := t.repo.Create(ctx, txNotification)
	return noti.ID, err
}

func (t *TxNotificationServiceV1) Commit(ctx context.Context, bizID int64, key string) error {
	noti, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return fmt.Errorf("查找事务失败 err:%w", err)
	}

	err = t.notificationSvc.BatchUpdateStatus(ctx, []uint64{noti.Notification.ID}, domain.SendStatusPending)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	err = t.repo.UpdateStatus(ctx, noti.TxID, domain.TxNotificationStatusCommit.String())
	// 你是要发送的，等我后续通知
	// t.sender.Send()
	if errors.Is(err, repository.ErrUpdateStatusFailed) {
		return ErrUpdateStatusFailed
	}
	return err
}

func (t *TxNotificationServiceV1) Cancel(ctx context.Context, bizID int64, key string) error {
	noti, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return fmt.Errorf("查找事务失败 err:%w", err)
	}

	err = t.notificationSvc.BatchUpdateStatus(ctx, []uint64{noti.Notification.ID}, domain.SendStatusCanceled)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	err = t.repo.UpdateStatus(ctx, noti.TxID, domain.TxNotificationStatusCancel.String())
	if errors.Is(err, repository.ErrUpdateStatusFailed) {
		return ErrUpdateStatusFailed
	}
	return err
}
