package testgrpc

import (
	"context"
	"log"
	"net"

	tx_notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/tx_notification/v1"
	"github.com/ego-component/eetcd/registry"
	"github.com/gotomicro/ego/core/constant"
	"github.com/gotomicro/ego/server"
	"google.golang.org/grpc"
)

type Server struct {
	name string
	*grpc.Server
	grpcServer tx_notificationv1.BackCheckServiceServer
	reg        *registry.Component
}

func NewServer(name string, reg *registry.Component, grpcServer tx_notificationv1.BackCheckServiceServer) *Server {
	return &Server{
		name:       name,
		reg:        reg,
		Server:     grpc.NewServer(),
		grpcServer: grpcServer,
	}
}

func (s *Server) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	err = s.reg.RegisterService(context.Background(), &server.ServiceInfo{
		Name:    s.name,
		Address: listener.Addr().String(),
		Scheme:  "grpc",
		Kind:    constant.ServiceProvider,
	})
	if err != nil {
		return err
	}
	tx_notificationv1.RegisterBackCheckServiceServer(s.Server, s.grpcServer)
	log.Println("grpc server register success")
	return s.Serve(listener)
}
