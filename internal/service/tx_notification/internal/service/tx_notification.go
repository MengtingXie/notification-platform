package service

import (
	"context"
	"fmt"

	"github.com/meoying/dlock-go"

	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service/retry"
	"github.com/gotomicro/ego/core/elog"
	"github.com/lithammer/shortuuid/v4"
)

type TxNotificationService interface {
	// Prepare 准备消息
	Prepare(ctx context.Context, txNotification domain.TxNotification) (string, error)
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
	task := &CheckBackTask{
		repo:                 t.repo,
		notificationSvc:      t.notificationSvc,
		configSvc:            t.configSvc,
		retryStrategyBuilder: t.retryStrategyBuilder,
		logger:               t.logger,
		lock:                 t.lock,
		batchSize:            defaultBatchSize,
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

func (t *TxNotificationServiceV1) Prepare(ctx context.Context, txNotification domain.TxNotification) (string, error) {
	noti, err := t.notificationSvc.CreateNotification(ctx, txNotification.Notification)
	if err != nil {
		return "", fmt.Errorf("创建通知失败 err:%w", err)
	}
	// 获取配置
	txNotification.TxID = shortuuid.New()
	txNotification.Notification = noti

	conf, err := t.configSvc.GetByID(ctx, txNotification.BizID)
	if err == nil {
		// 找到配置
		retryStrategy, rerr := t.retryStrategyBuilder.Build(conf.TxnConfig)
		if rerr != nil {
			return "", rerr
		}
		nextCheckTime, ok := retryStrategy.NextTime(retry.Req{
			CheckTimes: 0,
		})
		if ok {
			txNotification.NextCheckTime = nextCheckTime
		}
	}
	err = t.repo.Create(ctx, txNotification)
	if err != nil {
		return "", fmt.Errorf("创建通知失败 err:%w", err)
	}
	return txNotification.TxID, err
}

func (t *TxNotificationServiceV1) Commit(ctx context.Context, bizID int64, key string) error {
	noti, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return fmt.Errorf("查找事务失败 err:%w", err)
	}

	err = t.notificationSvc.UpdateNotificationStatus(ctx, noti.Notification.ID, notification.StatusSucceeded)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	return t.repo.UpdateStatus(ctx, []string{noti.TxID}, domain.TxNotificationStatusCommit.String())
}

func (t *TxNotificationServiceV1) Cancel(ctx context.Context, bizID int64, key string) error {
	noti, err := t.repo.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return fmt.Errorf("查找事务失败 err:%w", err)
	}

	err = t.notificationSvc.UpdateNotificationStatus(ctx, noti.Notification.ID, notification.StatusCanceled)
	if err != nil {
		return fmt.Errorf("更新事务失败 err:%w", err)
	}
	return t.repo.UpdateStatus(ctx, []string{noti.TxID}, domain.TxNotificationStatusCancel.String())
}
