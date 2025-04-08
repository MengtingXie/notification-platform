package provider

import (
	"context"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
)

// Selector 供应商选择器接口
type Selector interface {
	// Next 获取下一个供应商
	Next(ctx context.Context, notification domain.Notification) (Provider, error)
	// Reset 重置选择器状态
	Reset()
}

// SequentialSelector 供应商顺序选择器
type SequentialSelector struct {
	idx       int
	providers []Provider
}

func (r *SequentialSelector) Next(_ context.Context, _ domain.Notification) (Provider, error) {
	if len(r.providers) == r.idx {
		return nil, fmt.Errorf("%w", errs.ErrNoAvailableProvider)
	}

	p := r.providers[r.idx]
	r.idx++
	return p, nil
}

func (r *SequentialSelector) Reset() {
	r.idx = 0
}

type SelectorBuilder struct {
	providers []Provider
}

func NewSelectorBuilder(providers []Provider) *SelectorBuilder {
	return &SelectorBuilder{providers: providers}
}

func (s *SelectorBuilder) BuildSequentialSelector() Selector {
	return &SequentialSelector{
		providers: s.providers,
	}
}
