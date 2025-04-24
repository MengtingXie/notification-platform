package registry

import "context"

type Registry interface {
	Register(ctx context.Context, svc Service) error
	Deregister(ctx context.Context, svc Service) error
	Resolve(ctx context.Context, svcName string) ([]Service, error)
}

type Service struct {
	Name string
}
