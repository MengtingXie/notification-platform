package main

import (
	"context"

	ioc2 "gitee.com/flycash/notification-platform/internal/ioc"
	"go.opentelemetry.io/otel/sdk/trace"

	"gitee.com/flycash/notification-platform/cmd/platform/ioc"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
	"github.com/gotomicro/ego/server/egovernor"
)

func main() {
	// 创建 ego 应用实例
	egoApp := ego.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tp := ioc2.InitZipkinTracer()
	defer func(tp *trace.TracerProvider, ctx context.Context) {
		err := tp.Shutdown(ctx)
		if err != nil {
			elog.Error("Shutdown zipkinTracer", elog.FieldErr(err))
		}
	}(tp, ctx)
	app := ioc.InitGrpcServer()
	app.StartTasks(ctx)

	// 启动服务
	if err := egoApp.Serve(
		egovernor.Load("server.governor").Build(),
		func() server.Server {
			return app.GrpcServer
		}(),
	).Cron(app.Crons...).
		Run(); err != nil {
		elog.Panic("startup", elog.FieldErr(err))
	}
}
