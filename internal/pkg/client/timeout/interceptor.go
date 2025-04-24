package timeout

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const timeoutStr = "timeout"

// InjectorInterceptor 超时时间注入拦截器
func InjectorInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// 1. 从context提取超时时间
		if deadline, ok := ctx.Deadline(); ok {
			// 3. 注入metadata（兼容已有metadata）
			ctx = metadata.AppendToOutgoingContext(ctx, timeoutStr, fmt.Sprintf("%d", deadline.UnixMilli()))
		}
		// 4. 透传修改后的context
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
