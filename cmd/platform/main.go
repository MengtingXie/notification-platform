package main

import (
	"context"
	"gitee.com/flycash/notification-platform/cmd/platform/ioc"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
)

// go run --main
func main() {
	instance := ego.New()
	app := ioc.InitGrpcServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.StartTasks(ctx)
	if err := instance.Serve(app.GrpcServer).
		Cron(app.Crons...).
		Run(); err != nil {
		elog.Panic("startup", elog.Any("err", err))
	}
}
