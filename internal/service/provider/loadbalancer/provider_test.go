//go:build unit

package loadbalancer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"github.com/stretchr/testify/assert"
)

// MockHealthAwareProvider 是一个模拟的HealthAwareProvider实现
type MockHealthAwareProvider struct {
	name           string
	failCount      int32        // 用于控制连续失败次数
	mu             sync.RWMutex // 保护shouldFail和healthCheckErr
	shouldFail     bool         // 是否应该失败
	callCount      int32        // 调用计数
	healthyStatus  atomic.Bool
	healthCheckErr error
}

func NewMockHealthAwareProvider(name string, shouldFail bool) *MockHealthAwareProvider {
	m := &MockHealthAwareProvider{
		name:           name,
		shouldFail:     shouldFail,
		healthCheckErr: nil,
	}
	m.healthyStatus.Store(true)
	return m
}

func (m *MockHealthAwareProvider) Send(_ context.Context, notification domain.Notification) (domain.SendResponse, error) {
	atomic.AddInt32(&m.callCount, 1)

	m.mu.RLock()
	shouldFail := m.shouldFail
	m.mu.RUnlock()

	if shouldFail {
		atomic.AddInt32(&m.failCount, 1)
		return domain.SendResponse{}, errors.New("mock provider sending error")
	}

	return domain.SendResponse{
		NotificationID: notification.ID,
		Status:         domain.SendStatusSucceeded,
	}, nil
}

func (m *MockHealthAwareProvider) CheckHealth(context.Context) error {
	m.mu.RLock()
	err := m.healthCheckErr
	m.mu.RUnlock()
	return err
}

// 将失败的提供者设置为恢复
func (m *MockHealthAwareProvider) MarkRecovered() {
	m.mu.Lock()
	m.shouldFail = false
	m.healthCheckErr = nil
	m.mu.Unlock()
}

// TestProviderLoadBalancingAndHealthRecovery 测试负载均衡Provider的主流程
// 包括：
// 1. 正常的轮询发送
// 2. 当一个provider持续失败时，会被标记为不健康
// 3. 当不健康的provider恢复后，会被重新标记为健康
func TestProviderLoadBalancingAndHealthRecovery(t *testing.T) {
	t.Parallel()
	// 创建3个模拟的Provider，其中一个会持续失败
	provider1 := NewMockHealthAwareProvider("provider1", false)
	provider2 := NewMockHealthAwareProvider("provider2", true) // 这个会失败
	provider3 := NewMockHealthAwareProvider("provider3", false)

	providers := []provider.HealthAwareProvider{provider1, provider2, provider3}

	// 创建负载均衡Provider，设置缓冲区长度为30，这样3次失败后provider2会被标记为不健康
	lb := NewProvider(providers, 30)

	// 启动健康检查

	// 使用较短的检查间隔，以便测试能更快地进行
	go lb.MonitorProvidersHealth(t.Context(), 500*time.Millisecond)

	// 创建一个测试通知
	notification := domain.Notification{
		ID:        123,
		BizID:     456,
		Key:       "test-key",
		Channel:   domain.ChannelSMS,
		Receivers: []string{"13800138000"},
		Template: domain.Template{
			ID:        789,
			VersionID: 1,
			Params:    map[string]string{"code": "1234"},
		},
	}

	// 第1阶段：发送几次请求，此时provider2仍然是健康的，但会失败
	// 由于轮询，请求会分发到provider1、provider2、provider3
	for i := 0; i < 6; i++ {
		resp, err := lb.Send(t.Context(), notification)
		if err == nil {
			assert.Equal(t, notification.ID, resp.NotificationID)
			assert.Equal(t, domain.SendStatusSucceeded, resp.Status)
		} else {
			// 当轮到provider2时，发送会失败
			assert.Error(t, err)
		}
	}

	// 等待足够的时间，让provider2被标记为不健康
	// 这里我们多发送几次请求，确保provider2达到失败阈值
	for i := 0; i < 5; i++ {
		_, _ = lb.Send(t.Context(), notification)
	}

	// 第2阶段：当provider2不健康时，请求只会发送到provider1和provider3
	// 重置计数
	atomic.StoreInt32(&provider1.callCount, 0)
	atomic.StoreInt32(&provider2.callCount, 0)
	atomic.StoreInt32(&provider3.callCount, 0)

	// 发送10个请求，应该都成功，且只发给provider1和provider3
	for i := 0; i < 10; i++ {
		resp, err := lb.Send(t.Context(), notification)
		assert.NoError(t, err)
		assert.Equal(t, notification.ID, resp.NotificationID)
	}

	// 检查provider2没有被调用
	assert.Greater(t, atomic.LoadInt32(&provider1.callCount), int32(0))
	assert.Equal(t, int32(0), atomic.LoadInt32(&provider2.callCount))
	assert.Greater(t, atomic.LoadInt32(&provider3.callCount), int32(0))

	// 第3阶段：让provider2恢复
	provider2.MarkRecovered()

	// 等待健康检查将provider2重新标记为健康
	time.Sleep(1 * time.Second)

	// 重置计数
	atomic.StoreInt32(&provider1.callCount, 0)
	atomic.StoreInt32(&provider2.callCount, 0)
	atomic.StoreInt32(&provider3.callCount, 0)

	// 再次发送请求，此时所有provider都应该能收到请求
	for i := 0; i < 9; i++ {
		resp, err := lb.Send(t.Context(), notification)
		assert.NoError(t, err)
		assert.Equal(t, notification.ID, resp.NotificationID)
	}

	// 检查所有provider都被调用了
	assert.Greater(t, atomic.LoadInt32(&provider1.callCount), int32(0))
	assert.Greater(t, atomic.LoadInt32(&provider2.callCount), int32(0))
	assert.Greater(t, atomic.LoadInt32(&provider3.callCount), int32(0))
}
