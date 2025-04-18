// Provider 为供应商实现添加指标收集的装饰器
package metrics

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"github.com/prometheus/client_golang/prometheus"
)

// Provider 为供应商实现添加指标收集的装饰器
type Provider struct {
	provider            provider.Provider
	sendDurationSummary *prometheus.SummaryVec
	sendCounter         *prometheus.CounterVec
	sendStatusCounter   *prometheus.CounterVec
	name                string
}

// NewProvider 创建一个新的带有指标收集的供应商
func NewProvider(name string, p provider.Provider) *Provider {
	sendDurationSummary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "provider_send_duration_seconds",
			Help:       "供应商发送通知耗时统计（秒）",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			MaxAge:     time.Minute * 5,
		},
		[]string{"provider", "channel", "status"},
	)

	sendCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "provider_send_total",
			Help: "供应商发送通知总数",
		},
		[]string{"provider", "channel"},
	)

	sendStatusCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "provider_send_status_total",
			Help: "供应商发送通知状态统计",
		},
		[]string{"provider", "channel", "status"},
	)

	// 注册指标
	prometheus.MustRegister(sendDurationSummary, sendCounter, sendStatusCounter)

	return &Provider{
		provider:            p,
		sendDurationSummary: sendDurationSummary,
		sendCounter:         sendCounter,
		sendStatusCounter:   sendStatusCounter,
		name:                name,
	}
}

// Send 发送通知并记录指标
func (p *Provider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// 开始计时
	startTime := time.Now()

	// 累加发送计数
	p.sendCounter.WithLabelValues(
		p.name,
		string(notification.Channel),
	).Inc()

	// 调用底层供应商发送通知
	response, err := p.provider.Send(ctx, notification)

	// 计算耗时
	duration := time.Since(startTime).Seconds()

	// 记录发送状态
	p.sendStatusCounter.WithLabelValues(
		p.name,
		string(notification.Channel),
		string(response.Status),
	).Inc()

	// 记录耗时
	p.sendDurationSummary.WithLabelValues(
		p.name,
		string(notification.Channel),
		string(response.Status),
	).Observe(duration)

	return response, err
}
