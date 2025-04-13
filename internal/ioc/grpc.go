package ioc

import (
	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/log"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/observability"
	"github.com/ego-component/eetcd"
	"github.com/ego-component/eetcd/registry"
	"github.com/gotomicro/ego/client/egrpc/resolver"
	"github.com/gotomicro/ego/server/egrpc"
)

func InitGrpc(noserver *grpcapi.NotificationServer, etcdClient *eetcd.Component) *egrpc.Component {
	// 注册全局的注册中心
	reg := registry.Load("").Build(registry.WithClientEtcd(etcdClient))
	resolver.Register("etcd", reg)

	// 创建observability拦截器
	obsInterceptor := observability.New().Build()
	// 创建日志拦截器
	logInterceptor := log.New().Build()

	server := egrpc.Load("server.grpc").Build(
		egrpc.WithUnaryInterceptor(obsInterceptor, logInterceptor),
	)

	notificationv1.RegisterNotificationServiceServer(server.Server, noserver)
	notificationv1.RegisterNotificationQueryServiceServer(server.Server, noserver)

	return server
}
