package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ecodeclub/ekit/syncx"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/meoying/dlock-go"

	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service/retry"
	"github.com/gotomicro/ego/core/elog"
)

var ErrUpdateStatusFailed = errors.New("update status failed")

//go:generate mockgen -source=./tx_notification.go -destination=../../mocks/tx_notification.mock.go -package=txnotificationmocks -typed TxNotificationService
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
	repo                 repository.TxNotificationRepository
	notificationSvc      notification.Service
	configSvc            config.Service
	retryStrategyBuilder retry.Builder
	logger               *elog.Component
	lock                 dlock.Client
}

func NewTxNotificationService(
	repo repository.TxNotificationRepository,
	notificationSvc notification.Service,
	configSvc config.Service,
	retryStrategyBuilder retry.Builder,
	lock dlock.Client,
) *TxNotificationServiceV1 {
	return &TxNotificationServiceV1{
		repo:                 repo,
		notificationSvc:      notificationSvc,
		configSvc:            configSvc,
		retryStrategyBuilder: retryStrategyBuilder,
		logger:               elog.DefaultLogger,
		lock:                 lock,
	}
}

const defaultBatchSize = 10

func (t *TxNotificationServiceV1) StartTask(ctx context.Context) {
	task := &TxCheckTask{
		repo:                 t.repo,
		notificationSvc:      t.notificationSvc,
		configSvc:            t.configSvc,
		retryStrategyBuilder: t.retryStrategyBuilder,
		logger:               t.logger,
		lock:                 t.lock,
		batchSize:            defaultBatchSize,
		clientMap:            syncx.Map[string, *egrpc.Component]{},
	}
	go task.Start(ctx)
}

func (t *TxNotificationServiceV1) GetNotification(ctx context.Context, bizID int64, key string) (domain.TxNotification, error) {
	txn, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return domain.TxNotification{}, err
	}
	n, err := t.notificationSvc.GetNotificationByID(ctx, txn.Notification.ID)
	if err != nil {
		return domain.TxNotification{}, err
	}
	txn.Notification = n
	return txn, nil
}

func (t *TxNotificationServiceV1) Prepare(ctx context.Context, txNotification domain.TxNotification) (uint64, error) {
	noti, nerr := t.notificationSvc.CreateNotification(ctx, txNotification.Notification)
	if nerr != nil {
		if errors.Is(nerr, notification.ErrNotificationDuplicate) {
			txn, err := t.repo.GetByBizIDKey(ctx, txNotification.BizID, txNotification.Key)
			if err != nil {
				return 0, err
			}
			return txn.Notification.ID, nil
		}
		return 0, nerr
	}
	txNotification.Notification = noti
	conf, err := t.configSvc.GetByID(ctx, txNotification.BizID)
	if err == nil {
		// 找到配置
		retryStrategy, rerr := t.retryStrategyBuilder.Build(conf.TxnConfig)
		if rerr != nil {
			return 0, rerr
		}
		nextCheckTime, ok := retryStrategy.NextTime(retry.Req{
			CheckTimes: 0,
		})
		if ok {
			txNotification.NextCheckTime = nextCheckTime
		}
	}
	_, err = t.repo.Create(ctx, txNotification)
	return noti.ID, err
}

func (t *TxNotificationServiceV1) Commit(ctx context.Context, bizID int64, key string) error {
	noti, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return fmt.Errorf("查找事务失败 err:%w", err)
	}

	err = t.notificationSvc.UpdateNotificationStatus(ctx, noti.Notification.ID, notification.SendStatusPending)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	err = t.repo.UpdateStatus(ctx, noti.TxID, domain.TxNotificationStatusCommit.String())
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

	err = t.notificationSvc.UpdateNotificationStatus(ctx, noti.Notification.ID, notification.SendStatusCanceled)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	err = t.repo.UpdateStatus(ctx, noti.TxID, domain.TxNotificationStatusCancel.String())
	if errors.Is(err, repository.ErrUpdateStatusFailed) {
		return ErrUpdateStatusFailed
	}
	return err
}
