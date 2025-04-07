package main

import (
	"gitee.com/flycash/notification-platform/cmd/platform/ioc"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
)

func main() {
	if err := ego.New().Serve(func() server.Server {
		return ioc.InitGrpcServer().GrpcServer
	}()).Run(); err != nil {
		elog.Panic("startup", elog.Any("err", err))
	}
}
