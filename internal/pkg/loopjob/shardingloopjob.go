package loopjob

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotomicro/ego/core/elog"
	"github.com/meoying/dlock-go"
	"time"
)

type  CtxKey string

const  (
	TabSuffix CtxKey = "tabSuffix"
	DB CtxKey = "db"
	Table CtxKey = "table"
)

type ShardingLoopJob struct {
	db             string   // 库名
	baseKey        string   // 业务标识
	tabSuffixes    []string //表后缀
	dclient        dlock.Client
	logger         *elog.Component
	biz            func(ctx context.Context) error
	retryInterval  time.Duration
	defaultTimeout time.Duration
}

func NewShardingLoopJob(
	dclient dlock.Client,
	// 你要执行的业务。注意当 ctx 被取消的时候，就会退出全部循环
	biz func(ctx context.Context) error,
	db string,
	tabSuffixes []string,
) *ShardingLoopJob {
	const defaultTimeout = 3 * time.Second
	return newShardingLoopJobLoop(dclient, biz, db, tabSuffixes, time.Minute, defaultTimeout)
}

// ShardingLoopJobLoop 用于创建一个ShardingLoopJobLoop实例，允许指定重试间隔，便于测试
func newShardingLoopJobLoop(
	dclient dlock.Client,
	biz func(ctx context.Context) error,
	db string,
	tabSuffixes []string,
	retryInterval time.Duration,
	defaultTimeout time.Duration,
) *ShardingLoopJob {
	return &ShardingLoopJob{
		dclient:        dclient,
		db:             db,
		tabSuffixes:    tabSuffixes,
		logger:         elog.DefaultLogger,
		biz:            biz,
		retryInterval:  retryInterval,
		defaultTimeout: defaultTimeout,
	}
}
func (l *ShardingLoopJob) generateKey(tab string) string {
	return fmt.Sprintf("%s:%s:%s", l.baseKey, l.db, tab)
}

func (l *ShardingLoopJob) Run(ctx context.Context) {
	for {
		for idx := range l.tabSuffixes {
			suffix := l.tabSuffixes[idx]
			key := l.generateKey(suffix)
			// 强锁
			lock, err := l.dclient.NewLock(ctx, key, l.retryInterval)
			if err != nil {
				l.logger.Error("初始化分布式锁失败，重试",
					elog.Any("err", err))
				// 暂停一会
				time.Sleep(l.retryInterval)
				continue
			}

			lockCtx, cancel := context.WithTimeout(ctx, l.defaultTimeout)
			// 没有拿到锁，不管是系统错误，还是锁被人持有，都没有关系
			// 暂停一段时间之后继续
			err = lock.Lock(lockCtx)
			cancel()
			if err != nil {
				l.logger.Error("没有抢到分布式锁，系统出现问题", elog.Any("err", err))
				time.Sleep(l.retryInterval)
				continue
			}
			go l.tableLoop(l.ctxWithTabSuffixDB(ctx,suffix), lock)
		}
	}
}

func (l *ShardingLoopJob) tableLoop(ctx context.Context, lock dlock.Lock) {
	// 在这里执行业务
	err := l.bizLoop(ctx, lock)
	// 要么是续约失败，要么是 ctx 本身已经过期了
	if err != nil {
		l.logger.Error("执行业务失败，将执行重试", elog.FieldErr(err))
	}
	// 不管是什么原因，都要考虑释放分布式锁了
	// 要稍微摆脱 ctx 的控制，因为此时 ctx 可能被取消了
	unCtx, cancel := context.WithTimeout(context.Background(), l.defaultTimeout)
	//nolint:contextcheck // 这里必须使用 Background Context，因为原始 ctx 可能已被取消，但仍需尝试解锁操作。
	unErr := lock.Unlock(unCtx)
	cancel()
	if unErr != nil {
		l.logger.Error("释放分布式锁失败", elog.Any("err", unErr))
	}
	err = ctx.Err()
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// 被取消，那么就要跳出循环
		l.logger.Info("任务被取消，退出任务循环")
		return
	default:
		// 不可挽回的错误，后续考虑回去
		l.logger.Error("执行任务失败，将执行重试")
		time.Sleep(l.retryInterval)
	}
}

func (l *ShardingLoopJob) bizLoop(ctx context.Context, lock dlock.Lock) error {
	const bizTimeout = 50 * time.Second
	for {
		// 可以确保业务在分布式锁过期之前结束
		bizCtx, cancel := context.WithTimeout(ctx, bizTimeout)
		err := l.biz(bizCtx)
		cancel()
		if err != nil {
			l.logger.Error("业务执行失败", elog.FieldErr(err))
		}
		if ctx.Err() != nil {
			// 要中断这个循环了
			return ctx.Err()
		}
		refCtx, cancel := context.WithTimeout(ctx, l.defaultTimeout)
		err = lock.Refresh(refCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("分布式锁续约失败 %w", err)
		}
	}
}

func (l *ShardingLoopJob) ctxWithTabSuffixDB(ctx context.Context, tabSuffix string) context.Context {
	ctx = context.WithValue(ctx, TabSuffix, tabSuffix)
	ctx = context.WithValue(ctx, DB, l.db)
	return ctx
}



func DBFromCtx(ctx context.Context) (string, bool) {
	db := ctx.Value(DB)
	if db == nil {
		return "", false
	}
	dbStr, ok := db.(string)
	return dbStr, ok
}

func TabSuffixFromCtx(ctx context.Context) (string, bool) {
	tab := ctx.Value(TabSuffix)
	if tab == nil {
		return "", false
	}
	tabStr, ok := tab.(string)
	return tabStr, ok
}

