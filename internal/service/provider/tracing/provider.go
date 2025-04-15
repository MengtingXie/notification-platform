package tracing

import (
	"context"
	"strconv"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Provider 为供应商实现添加链路追踪的装饰器
type Provider struct {
	provider provider.Provider
	tracer   trace.Tracer
}

// NewProvider 创建一个新的带有链路追踪的供应商
func NewProvider(p provider.Provider) *Provider {
	return &Provider{
		provider: p,
		tracer:   otel.Tracer("notification-platform/provider"),
	}
}

func (p *Provider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	ctx, span := p.tracer.Start(ctx, "Provider.Send",
		trace.WithAttributes(
			attribute.String("notification.id", strconv.FormatUint(notification.ID, 10)),
			attribute.String("notification.bizId", strconv.FormatInt(notification.BizID, 10)),
			attribute.String("notification.key", notification.Key),
			attribute.String("notification.channel", string(notification.Channel)),
		))
	defer span.End()

	response, err := p.provider.Send(ctx, notification)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(
			attribute.String("notification.id", strconv.FormatUint(response.NotificationID, 10)),
			attribute.String("notification.status", string(response.Status)),
		)
	}

	return response, err
}
