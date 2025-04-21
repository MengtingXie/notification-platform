package limit

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/pkg/config"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
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
	limiter ratelimit.Limiter
	key     string

	failoverMgr config.FailoverManager
	svcInstance config.ServiceInstance
	limited     bool
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}
		if limited {
			// 触发限流后，标记故障转移
			err1 := b.failoverMgr.Failover(ctx, b.svcInstance)
			if err1 != nil {
				return nil, err1
			}
			b.limited = true
			return nil, status.Errorf(codes.ResourceExhausted, errs.ErrRateLimited.Error())
		} else {
			if b.limited {
				// 限流解除后，标记已恢复
				err2 := b.failoverMgr.Recover(ctx, b.svcInstance)
				if err2 != nil {
					return nil, err2
				}
				b.limited = false
			}
		}
		return handler(ctx, req)
	}
}
