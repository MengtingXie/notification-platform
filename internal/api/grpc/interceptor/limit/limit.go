package limit

import (
	"context"
	"sync/atomic"

	"gitee.com/flycash/notification-platform/internal/errs"
	ratelimitevt "gitee.com/flycash/notification-platform/internal/event/ratelimit"
	"gitee.com/flycash/notification-platform/internal/pkg/config"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	"github.com/gotomicro/ego/core/elog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// func LimitAuthInterceptor() grpc.UnaryServerInterceptor {
//	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
//
//		if check() {
//			ctx = context.WithValue(ctx, "rate_limit", true)
//		}
//
//	}
// }
//
// func check() bool {
//	// 在这里引入令牌桶或者漏桶算法，或者我们第三周讲的高端的限流判定
// }

// Builder 限流拦截器构建器
type Builder struct {
	svcInstance config.ServiceInstance

	limitedKey     string
	limiter        ratelimit.Limiter
	inLimitedState atomic.Bool

	failoverMgr config.FailoverManager
	producer    ratelimitevt.RequestRateLimitedEventProducer

	logger *elog.Component
}

func NewBuilder(
	svcInstance config.ServiceInstance,
	limitedKey string,
	limiter ratelimit.Limiter,
	failoverMgr config.FailoverManager,
	producer ratelimitevt.RequestRateLimitedEventProducer,
) *Builder {
	return &Builder{
		svcInstance: svcInstance,
		limitedKey:  limitedKey,
		limiter:     limiter,
		failoverMgr: failoverMgr,
		producer:    producer,
		logger:      elog.DefaultLogger,
	}
}

func (b *Builder) Build() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		limited, err := b.limiter.Limit(ctx, b.limitedKey)
		if err != nil {
			// 保守策略
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}
		if limited {
			// 触发限流标记限流状态
			if b.inLimitedState.CompareAndSwap(false, true) {
				// 标记故障转移
				err1 := b.failoverMgr.Failover(ctx, b.svcInstance)
				if err1 != nil {
					// 只记录日志
					b.logger.Error("故障转移失败",
						elog.FieldErr(err1),
						elog.Any("ServiceInstance", b.svcInstance),
					)
					b.inLimitedState.Store(false)
				}
			}
			// 被限流请求转存MQ
			err2 := b.producer.Produce(ctx, ratelimitevt.RequestRateLimitedEvent{Request: req})
			if err2 != nil {
				// 只记录日志
				b.logger.Error("转存请求失败",
					elog.FieldErr(err2),
					elog.Any("req", req),
					elog.Any("info", info))
			}
			// 请求被限流
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}

		// 之前处于限流状态
		if b.inLimitedState.CompareAndSwap(true, false) {
			// 限流解除后，标记已恢复
			err3 := b.failoverMgr.Recover(ctx, b.svcInstance)
			if err3 != nil {
				// 只记录日志
				b.logger.Error("故障恢复失败",
					elog.FieldErr(err3),
					elog.Any("ServiceInstance", b.svcInstance),
				)
				b.inLimitedState.Store(true)
			}
		}

		return handler(ctx, req)
	}
}
