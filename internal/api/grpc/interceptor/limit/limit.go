package limit

//func LimitAuthInterceptor() grpc.UnaryServerInterceptor {
//	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
//
//		if check() {
//			ctx = context.WithValue(ctx, "rate_limit", true)
//		}
//
//	}
//}
//
//func check() bool {
//	// 在这里引入令牌桶或者漏桶算法，或者我们第三周讲的高端的限流判定
//}
