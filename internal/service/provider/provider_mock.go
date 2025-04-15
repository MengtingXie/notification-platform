package provider

import (
	"context"
	"sync/atomic"

	"gitee.com/flycash/notification-platform/internal/domain"
)

type MockProvider struct {
	count int64
}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	v := atomic.AddInt64(&m.count, 1)
	return domain.SendResponse{
		Status:         domain.SendStatusSucceeded,
		NotificationID: uint64(v),
	}, nil
}
