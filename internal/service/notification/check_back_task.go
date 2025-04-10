package notification

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"strings"
	"sync"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"

	"github.com/ecodeclub/ekit/syncx"

	clientv1 "gitee.com/flycash/notification-platform/api/proto/gen/client/v1"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
	"golang.org/x/sync/errgroup"
)

type TxCheckTask struct {
	repo            repository.TxNotificationRepository
	notificationSvc Service
	configSvc       config.BusinessConfigService
	logger          *elog.Component
	lock            dlock.Client
	batchSize       int
	clientMap       syncx.Map[string, *egrpc.Component]
}

const (
	TxCheckTaskKey  = "check_back_job"
	defaultTimeout  = 5 * time.Second
	committedStatus = 1
	unknownStatus   = 0
	cancelStatus    = 2
)

func (task *TxCheckTask) Start(ctx context.Context) {
	job := loopjob.NewInfiniteLoop(task.lock, task.oneLoop, TxCheckTaskKey)
	job.Run(ctx)
}

func (task *TxCheckTask) oneLoop(ctx context.Context) error {
	loopCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	txNotifications, err := task.repo.FindCheckBack(loopCtx, 0, task.batchSize)
	if err != nil {
		return err
	}

	if len(txNotifications) == 0 {
		// 避免立刻又调度
		time.Sleep(time.Second)
		return nil
	}

	bizIDs := slice.Map(txNotifications, func(_ int, src domain.TxNotification) int64 {
		return src.BizID
	})
	configMap, err := task.configSvc.GetByIDs(loopCtx, bizIDs)
	if err != nil {
		return err
	}
	var eg errgroup.Group
	mu := &sync.Mutex{}
	// 需要继续重试的事务通知,
	retryTxns := make([]domain.TxNotification, 0, len(txNotifications))
	// 需要提交的事务通知
	commitTxns := make([]domain.TxNotification, 0, len(txNotifications))
	// 失败和取消的事务通知
	failedTxns := make([]domain.TxNotification, 0, len(txNotifications))

	for idx := range txNotifications {
		eg.Go(func() error {
			txNotification := txNotifications[idx]
			txn := task.oneBackCheck(ctx, configMap, txNotification)
			switch txn.Status {
			case domain.TxNotificationStatusCommit:
				mu.Lock()
				commitTxns = append(commitTxns, txn)
				mu.Unlock()
			case domain.TxNotificationStatusFail:
				mu.Lock()
				failedTxns = append(failedTxns, txn)
				mu.Unlock()
			case domain.TxNotificationStatusPrepare:
				mu.Lock()
				retryTxns = append(retryTxns, txn)
				mu.Unlock()
			case domain.TxNotificationStatusCancel:
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
func (task *TxCheckTask) oneBackCheck(ctx context.Context, configMap map[int64]domain.BusinessConfig, txNotification domain.TxNotification) domain.TxNotification {
	conf, ok := configMap[txNotification.BizID]
	if !ok {
		// 没设置，所以不需要回查，在不需要回查的情况下，事务又很久没有提交，标记为失败
		txNotification.NextCheckTime = 0
		txNotification.Status = domain.TxNotificationStatusFail
		return txNotification
	}
	res, err := task.getCheckBackRes(ctx, *conf.TxnConfig, txNotification)
	// 都检查了一次无论成功与否
	txNotification.CheckCount++
	if err != nil || res == unknownStatus {
		// 进行重试
		txNotification.SetNextCheckBackTimeAndStatus(conf.TxnConfig)
		return txNotification
	}

	if res == cancelStatus {
		txNotification.Status = domain.TxNotificationStatusCancel
		txNotification.NextCheckTime = 0
	}
	if res == committedStatus {
		txNotification.NextCheckTime = 0
		txNotification.Status = domain.TxNotificationStatusCommit
	}
	return txNotification
}

func (task *TxCheckTask) getCheckBackRes(ctx context.Context, conf domain.TxnConfig, txn domain.TxNotification) (status int, err error) {
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
	client := clientv1.NewTransactionCheckServiceClient(grpcConn)
	// 准备请求参数
	req := &clientv1.TransactionCheckServiceCheckRequest{
		Key: txn.Key,
	}

	// 发起远程调用
	resp, err := client.Check(ctx, req)
	if err != nil {
		return 0, err
	}

	return int(resp.Status), nil
}

func (task *TxCheckTask) commit(ctx context.Context, txns []domain.TxNotification) {
	err := task.repo.UpdateCheckStatus(ctx, txns, string(domain.SendStatusPending))
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) fail(ctx context.Context, txns []domain.TxNotification) {
	err := task.repo.UpdateCheckStatus(ctx, txns, string(domain.SendStatusCanceled))
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) retry(ctx context.Context, txns []domain.TxNotification) {
	err := task.repo.UpdateCheckStatus(ctx, txns, string(domain.SendStatusPrepare))
	if err != nil {
		task.logger.Error("更新事务表中数据失败", elog.String("tx_ids", task.taskIDs(txns)), elog.FieldErr(err))
	}
}

func (task *TxCheckTask) taskIDs(txns []domain.TxNotification) string {
	txids := slice.Map(txns, func(_ int, src domain.TxNotification) string {
		return fmt.Sprintf("%d", src.TxID)
	})
	return strings.Join(txids, ",")
}
