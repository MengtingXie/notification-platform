package registry

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/hashicorp/go-multierror"
)

type MultipleRegistry struct {
	registries []Registry
	main       RegistryStatus
	back       RegistryStatus
}

// RegistryStatus 增加心跳方法，然后心跳不通，就把 active 设置为 false
type RegistryStatus struct {
	r      Registry
	active atomic.Bool
}

func (m *MultipleRegistry) ResolveV1(ctx context.Context, svcName string) ([]Service, error) {
	if m.main.active.Load() {
		return m.main.r.Resolve(ctx, svcName)
	}
	return m.back.r.Resolve(ctx, svcName)
}

func (m *MultipleRegistry) Resolve(ctx context.Context, svcName string) ([]Service, error) {
	for _, r := range m.registries {
		svcs, err := r.Resolve(ctx, svcName)
		if err == nil {
			return svcs, nil
		}
	}
	return nil, errors.New("所有注册中心都失败了")
}

func (m *MultipleRegistry) Register(ctx context.Context, svc Service) error {
	for _, r := range m.registries {
		err := r.Register(ctx, svc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MultipleRegistry) Deregister(ctx context.Context, svc Service) error {
	var err error
	for _, r := range m.registries {
		err = multierror.Append(err, r.Deregister(ctx, svc))
		// 你可以考虑记录日志，Deregister != nil 的时候
	}
	return err
}

type MultipleRegistryV1 struct {
	// mainClient   *etcdv3.Client
	// backupClient *etcdv3.Client
}
