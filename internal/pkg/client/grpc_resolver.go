package client

import (
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/registry"
	"golang.org/x/net/context"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

type grpcResolverBuilder struct {
	r       registry.Registry
	timeout time.Duration
}

func NewResolverBuilder(r registry.Registry, timeout time.Duration) resolver.Builder {
	return &grpcResolverBuilder{
		r:       r,
		timeout: timeout,
	}
}

func (r *grpcResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	res := &grpcResolver{
		target:   target,
		cc:       cc,
		registry: r.r,
		close:    make(chan struct{}, 1),
		timeout:  r.timeout,
	}
	res.resolve()
	go res.watch()
	return res, nil
}

func (r *grpcResolverBuilder) Scheme() string {
	return "registry"
}

type grpcResolver struct {
	target   resolver.Target
	cc       resolver.ClientConn
	registry registry.Registry
	close    chan struct{}
	timeout  time.Duration
}

func (g *grpcResolver) ResolveNow(_ resolver.ResolveNowOptions) {
	// 重新获取一下所有服务
	g.resolve()
}

func (g *grpcResolver) Close() {
	g.close <- struct{}{}
}

func (g *grpcResolver) watch() {
	events := g.registry.Subscribe(g.target.Endpoint())
	for {
		select {
		case <-events:
			g.resolve()

		case <-g.close:
			return
		}
	}
}

func (g *grpcResolver) resolve() {
	serviceName := g.target.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	instances, err := g.registry.ListServices(ctx, serviceName)
	cancel()
	if err != nil {
		g.cc.ReportError(err)
	}

	address := make([]resolver.Address, 0, len(instances))
	for _, ins := range instances {
		address = append(address, resolver.Address{
			Addr:       ins.Address,
			ServerName: ins.Name,
			Attributes: attributes.New(readWeightStr, ins.ReadWeight).
				WithValue(writeWeightStr, ins.WriteWeight).
				WithValue(groupStr, ins.Group).
				WithValue(nodeStr, ins.Name),
		})
	}
	err = g.cc.UpdateState(resolver.State{
		Addresses: address,
	})
	if err != nil {
		g.cc.ReportError(err)
	}
}
