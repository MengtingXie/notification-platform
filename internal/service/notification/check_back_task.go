package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gotomicro/ego/client/egrpc"

	"gitee.com/flycash/notification-platform/internal/pkg/grpc"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"github.com/ecodeclub/ekit/list"
	"github.com/hashicorp/go-multierror"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"

	clientv1 "gitee.com/flycash/notification-platform/api/proto/gen/client/v1"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
	"golang.org/x/sync/errgroup"
)

type TxCheckTask struct {
	repo      repository.TxNotificationRepository
	configSvc config.BusinessConfigService
	logger    *elog.Component
	lock      dlock.Client
	batchSize int
	clients   *grpc.Clients[clientv1.TransactionCheckServiceClient]
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

func NewTask(repo repository.TxNotificationRepository,
	configSvc config.BusinessConfigService,
	lock dlock.Client,
) *TxCheckTask {
	return &TxCheckTask{
		repo:      repo,
		configSvc: configSvc,
		lock:      lock,
		batchSize: defaultBatchSize,
		logger:    elog.DefaultLogger,
		clients: grpc.NewClients[clientv1.TransactionCheckServiceClient](func(conn *egrpc.Component) clientv1.TransactionCheckServiceClient {
			return clientv1.NewTransactionCheckServiceClient(conn)
		}),
	}
}

// 为了性能，使用了批量操作，针对的是数据库的批量操作
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
	configMap, err := task.configSvc.GetByIDs(ctx, bizIDs)
	length := len(txNotifications)
	// 这一次回查没拿到明确结果的
	retryTxns := &list.ConcurrentList[domain.TxNotification]{
		List: list.NewArrayList[domain.TxNotification](length),
	}

	// 要回滚的
	failTxns := &list.ConcurrentList[domain.TxNotification]{
		List: list.NewArrayList[domain.TxNotification](length),
	}
	// 要提交的
	commitTxns := &list.ConcurrentList[domain.TxNotification]{
		List: list.NewArrayList[domain.TxNotification](length),
	}
	// 挨个处理呀
	var eg errgroup.Group
	for idx := range txNotifications {
		idx := idx
		eg.Go(func() error {
			// 并发去回查
			txNotification := txNotifications[idx]
			// 我在这里发起了回查，而后拿到了结果
			txn := task.oneBackCheck(ctx, configMap, txNotification)
			switch txn.Status {
			case domain.TxNotificationStatusPrepare:
				// 查到还是 Prepare 状态
				_ = retryTxns.Append(txn)
			case domain.TxNotificationStatusFail, domain.TxNotificationStatusCancel:
				_ = failTxns.Append(txn)
			case domain.TxNotificationStatusCommit:
				_ = commitTxns.Append(txn)
			default:
				return errors.New("不合法的回查状态")
			}
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return err
	}
	// 挨个处理，更新数据库状态
	// 数据库就可以一次性执行完，规避频繁更新数据库
	err = task.updateStatus(ctx, retryTxns, domain.SendStatusPrepare)
	err = multierror.Append(err, task.updateStatus(ctx, failTxns, domain.SendStatusFailed))
	// 转 PENDING，后续 Scheduler 会调度执行
	err = multierror.Append(err, task.updateStatus(ctx, commitTxns, domain.SendStatusPending))
	return err
}

// 校验完了
func (task *TxCheckTask) oneBackCheck(ctx context.Context, configMap map[int64]domain.BusinessConfig, txNotification domain.TxNotification) domain.TxNotification {
	bizConfig, ok := configMap[txNotification.BizID]
	if !ok || bizConfig.TxnConfig == nil {
		// 没设置，不需要回查
		txNotification.NextCheckTime = 0
		txNotification.Status = domain.TxNotificationStatusFail
		return txNotification
	}

	txConfig := bizConfig.TxnConfig
	// 发起回查
	res, err := task.getCheckBackRes(ctx, *txConfig, txNotification)
	// 执行了一次回查，要 +1
	txNotification.CheckCount++
	// 回查失败了
	if err != nil || res == unknownStatus {
		// 重新计算下一次的回查时间
		txNotification.SetNextCheckBackTimeAndStatus(txConfig)
		return txNotification
	}
	switch res {
	case cancelStatus:
		txNotification.NextCheckTime = 0
		txNotification.Status = domain.TxNotificationStatusCancel
	case committedStatus:
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
	// 借助服务发现来回查
	client := task.clients.Get(conf.ServiceName)

	req := &clientv1.TransactionCheckServiceCheckRequest{Key: txn.Key}
	resp, err := client.Check(ctx, req)
	if err != nil {
		return unknownStatus, err
	}
	return int(resp.Status), nil
}

func (task *TxCheckTask) updateStatus(ctx context.Context,
	list *list.ConcurrentList[domain.TxNotification], status domain.SendStatus,
) error {
	if list.Len() == 0 {
		return nil
	}
	txns := list.AsSlice()
	return task.repo.UpdateCheckStatus(ctx, txns, status)
}
