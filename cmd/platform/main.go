package main

import (
	"context"

	"gitee.com/flycash/notification-platform/cmd/platform/ioc"
	ioc2 "gitee.com/flycash/notification-platform/internal/ioc"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
	"github.com/gotomicro/ego/server/egovernor"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// 创建 ego 应用实例
	app := ioc.InitGrpcServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.StartTasks(ctx)

	// 初始化 zipkin trace
	tp := ioc2.InitZipkinTracer()
	defer func(tp *trace.TracerProvider, ctx context.Context) {
		err := tp.Shutdown(ctx)
		if err != nil {
			elog.Error("Shutdown zipkinTracer", elog.FieldErr(err))
		}
	}(tp, ctx)

	// 启动服务
	if err := ego.New().Serve(
		egovernor.Load("server.governor").Build(),
		func() server.Server {
			return app.GrpcServer
		}(),
	).Cron(app.Crons...).
		Run(); err != nil {
		elog.Panic("startup", elog.FieldErr(err))
	}
}
