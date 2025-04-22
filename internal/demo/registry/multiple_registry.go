package registry

import "context"

type MultipleRegistry struct {
	registries []Registry
}

func (m *MultipleRegistry) Register(ctx context.Context, svc Service) error {
	for _, r := range m.registries {
		// 直接中断。没必要再去 Deregister 已经注册成功的
		err := r.Register(ctx, svc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MultipleRegistry) Deregister(ctx context.Context, svc Service) error {
	// 尽可能完成的
	for _, r := range m.registries {
		err := r.Deregister(ctx, svc)
		if err != nil {
			// 记录日志
			continue
		}
	}
	return nil
}
