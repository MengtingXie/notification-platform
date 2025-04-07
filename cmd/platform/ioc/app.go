package ioc

import (
	"github.com/gotomicro/ego/server/egrpc"
)

type App struct {
	GrpcServer *egrpc.Component
}
