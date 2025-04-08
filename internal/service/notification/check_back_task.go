package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	repository2 "gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"strings"
	"sync"
	"time"

	"github.com/ecodeclub/ekit/syncx"

	tx_notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/client/v1"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service/retry"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
	"golang.org/x/sync/errgroup"
)

type TxCheckTask struct {
	repo                 repository.TxNotificationRepository
	notificationSvc      Service
	configSvc            config.Service
	retryStrategyBuilder retry.Builder
	logger               *elog.Component
	lock                 dlock.Client
	batchSize            int
	clientMap            syncx.Map[string, *egrpc.Component]
}

const (
	TxCheckTaskKey  = "check_back_job"
	defaultTimeout  = 5 * time.Second
	committedStatus = 1
	unknownStatus   = 0
	cancelStatus    = 2
)

func (task *TxCheckTask) Start(ctx context.Context) {
	task.logger = task.logger.With(elog.String("key", TxCheckTaskKey))
	interval := time.Minute
	for {
		// 每个循环过程就是一次尝试拿到分布式锁之后，不断调度的过程
		lock, err := task.lock.NewLock(ctx, TxCheckTaskKey, interval)
		if err != nil {
			task.logger.Error("初始化分布式锁失败，重试",
				elog.Any("err", err))
			// 暂停一会
			time.Sleep(interval)
			continue
		}

		lockCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
		// 没有拿到锁，不管是系统错误，还是锁被人持有，都没有关系
		// 暂停一段时间之后继续
		err = lock.Lock(lockCtx)
		cancel()

		if err != nil {
			if errors.Is(err, dlock.ErrLocked) {
				task.logger.Info("没有抢到分布式锁，此刻正有人持有锁")
			} else {
				task.logger.Error("没有抢到分布式锁，系统出现问题", elog.Any("err", err))
			}
			// 10 秒钟是一个比较合适的
			time.Sleep(interval)
			continue
		}

		err = task.loop(ctx)
		if err != nil {
			task.logger.Error("回查失败", elog.Any("err", err))
		}
		if unErr := lock.Unlock(ctx); unErr != nil {
			task.logger.Error("释放分布式锁失败", elog.Any("err", unErr))
		}
		// 从这里退出的时候，要检测一下是不是需要结束了
		ctxErr := ctx.Err()
		switch {
		case errors.Is(ctxErr, context.Canceled), errors.Is(ctxErr, context.DeadlineExceeded):
			// 被取消，那么就要跳出循环
			task.logger.Info("任务被取消，退出任务循环")
			return
		default:
			task.logger.Error("执行回查任务失败，将执行重试")
			time.Sleep(interval)
		}
	}
}

func (task *TxCheckTask) loop(ctx context.Context) error {
	loopCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	txNotifications, err := task.repo.Find(loopCtx, 0, task.batchSize)
	if err != nil {
		return err
	}
	bizIDs := slice.Map(txNotifications, func(_ int, src repository2.TxNotification) int64 {
		return src.BizID
	})
	configMap, err := task.configSvc.GetByIDs(loopCtx, bizIDs)
	if err != nil {
		return err
	}
	var eg errgroup.Group
	mu := &sync.Mutex{}
	// 需要继续重试的事务通知,
	retryTxns := make([]repository2.TxNotification, 0, len(txNotifications))
	// 需要提交的事务通知
	commitTxns := make([]repository2.TxNotification, 0, len(txNotifications))
	// 失败和取消的事务通知
	failedTxns := make([]repository2.TxNotification, 0, len(txNotifications))

	for idx := range txNotifications {
		eg.Go(func() error {
			txNotification := txNotifications[idx]
			txn := task.oneBackCheck(ctx, configMap, txNotification)
			switch txn.Status {
			case repository2.TxNotificationStatusCommit:
				mu.Lock()
				commitTxns = append(commitTxns, txn)
				mu.Unlock()
			case repository2.TxNotificationStatusFail:
				mu.Lock()
				failedTxns = append(failedTxns, txn)
				mu.Unlock()
			case repository2.TxNotificationStatusPrepare:
				mu.Lock()
				retryTxns = append(retryTxns, txn)
				mu.Unlock()
			case repository2.TxNotificationStatusCancel:
				mu.Lock()
				failedTxns = append(failedTxns, txn)
				mu.Unlock()
			default:
				return errors.New("unexpected status")
			}
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return err
	}
	if len(retryTxns) > 0 {
		task.retry(ctx, retryTxns)
	}
	if len(commitTxns) > 0 {
		task.commit(ctx, commitTxns)
	}
	if len(failedTxns) > 0 {
		task.fail(ctx, failedTxns)
	}
	return nil
}

// 校验完了
func (task *TxCheckTask) oneBackCheck(ctx context.Context, configMap map[int64]config.BusinessConfig, txNotification repository2.TxNotification) repository2.TxNotification {
	conf, ok := configMap[txNotification.BizID]
	if !ok || conf.TxnConfig == "" {
		// 没设置，所以不需要回查
		txNotification.NextCheckTime = 0
		return txNotification
	}
	backConfig, err := task.getConfigFromStr(conf.TxnConfig)
	if err != nil {
		// 配置解析失败也不需要回查了,等他自己取消，提交
		txNotification.NextCheckTime = 0
		return txNotification
	}
	res, err := task.getCheckBackRes(ctx, backConfig, txNotification)
	// 都检查了一次无论成功与否
	txNotification.CheckCount++
	if err != nil || res == unknownStatus {
		builder, verr := task.retryStrategyBuilder.Build(conf.TxnConfig)
		if verr != nil {
			txNotification.NextCheckTime = 0
			return txNotification
		}
		// 进行重试
		nextTime, ok := builder.NextTime(retry.Req{
			CheckTimes: txNotification.CheckCount,
		})
		// 可以重试
		if ok {
			txNotification.NextCheckTime = nextTime
			return txNotification
		}
		// 不能重试将状态变成fail
		txNotification.NextCheckTime = 0
		txNotification.Status = repository2.TxNotificationStatusFail
		return txNotification

	}

	if res == cancelStatus {
		txNotification.Status = repository2.TxNotificationStatusCancel
		txNotification.NextCheckTime = 0
	}
	if res == committedStatus {
		txNotification.NextCheckTime = 0
		txNotification.Status = repository2.TxNotificationStatusCommit
	}
	return txNotification
}

type CheckBackConfig struct {
	ServiceName string `json:"serviceName"`
}

func (task *TxCheckTask) getConfigFromStr(conf string) (CheckBackConfig, error) {
	var cfg CheckBackConfig
	err := json.Unmarshal([]byte(conf), &cfg)
	if err != nil {
		return CheckBackConfig{}, err
	}
	return cfg, nil
}

func (task *TxCheckTask) getCheckBackRes(ctx context.Context, conf CheckBackConfig, txn repository2.TxNotification) (status int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if str, ok := r.(string); ok {
				err = errors.New(str)
			} else {
				err = fmt.Errorf("未知panic类型: %v", r)
			}
		}
	}()

	// 使用ego的服务发现，根据conf的servicename 来访问对应的服务使用notification-platform/api/proto/gen/tx_notification/v1下的服务
	grpcConn, ok := task.clientMap.Load(conf.ServiceName)
	if !ok {
		grpcConn = egrpc.Load("").Build(egrpc.WithAddr(fmt.Sprintf("etcd:///%s", conf.ServiceName)))
		task.clientMap.Store(conf.ServiceName, grpcConn)
	}
	// 创建BackCheckService的客户端
	client := tx_notificationv1.NewTransactionCheckServiceClient(grpcConn)
	// 准备请求参数
	req := &tx_notificationv1.TransactionCheckServiceCheckRequest{
		Key: txn.Key,
	}

	// 发起远程调用
	resp, err := client.Check(ctx, req)
	if err != nil {
		return 0, err
	}

	return int(resp.Status), nil
}

func (task *TxCheckTask) commit(ctx context.Context, txns []repository2.TxNotification) {
	// 更新远程
	ids := slice.Map(txns, func(_ int, src repository2.TxNotification) uint64 {
		return src.Notification.ID
	})
	err := task.notificationSvc.BatchUpdateStatus(ctx, ids, SendStatusPending)
	if err != nil {
		task.logger.Error("更新通知服务中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
		return
	}
	err = task.repo.UpdateCheckStatus(ctx, txns)
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) fail(ctx context.Context, txns []repository2.TxNotification) {
	ids := slice.Map(txns, func(_ int, src repository2.TxNotification) uint64 {
		return src.Notification.ID
	})
	err := task.notificationSvc.BatchUpdateStatus(ctx, ids, SendStatusCanceled)
	if err != nil {
		task.logger.Error("更新通知服务中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
		return
	}
	err = task.repo.UpdateCheckStatus(ctx, txns)
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) retry(ctx context.Context, txns []repository2.TxNotification) {
	err := task.repo.UpdateCheckStatus(ctx, txns)
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) taskIDs(txns []repository2.TxNotification) string {
	txids := slice.Map(txns, func(_ int, src repository2.TxNotification) string {
		return fmt.Sprintf("%d", src.TxID)
	})
	return strings.Join(txids, ",")
}
