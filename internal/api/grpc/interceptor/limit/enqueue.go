package limit

import (
	"context"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/jwt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	ratelimitevt "gitee.com/flycash/notification-platform/internal/event/ratelimit"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	"github.com/gotomicro/ego/core/elog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnqueueRateLimitedRequestBuilder 限流请求入队拦截器构建器
type EnqueueRateLimitedRequestBuilder struct {
	limitedKey string
	limiter    ratelimit.Limiter

	producer    ratelimitevt.RequestRateLimitedEventProducer
	templateSvc templatesvc.ChannelTemplateService

	logger *elog.Component
}

func NewEnqueueRateLimitedRequestBuilder(
	limitedKey string,
	limiter ratelimit.Limiter,
	producer ratelimitevt.RequestRateLimitedEventProducer,
	templateSvc templatesvc.ChannelTemplateService,
) *EnqueueRateLimitedRequestBuilder {
	return &EnqueueRateLimitedRequestBuilder{
		limitedKey:  limitedKey,
		limiter:     limiter,
		producer:    producer,
		templateSvc: templateSvc,
		logger:      elog.DefaultLogger,
	}
}

func (b *EnqueueRateLimitedRequestBuilder) Build() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		limited, err := b.limiter.Limit(ctx, b.limitedKey)
		if err != nil {
			// 保守策略
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}

		// 未限流，继续流程
		if !limited {
			return handler(ctx, req)
		}

		// 已限流，判断是否转存请求 —— 通知写请求
		notificationHandler, ok := req.(notificationv1.NotificationHandler)
		if !ok {
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}

		bizID, err2 := jwt.GetBizIDFromContext(ctx)
		if err != nil {
			b.logger.Warn("获取BizID失败",
				elog.FieldErr(err2),
				elog.Any("req", req),
				elog.Any("info", info))
			return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
		}

		ns := notificationHandler.GetNotifications()
		domainNotifications := make([]domain.Notification, len(ns))
		for i := range ns {
			notification, err3 := ns[i].ToDomainNotification()
			if err3 != nil {
				b.logger.Warn("转换为domain.NotificationB失败",
					elog.FieldErr(err3),
					elog.Any("req", req),
					elog.Any("info", info))
				return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
			}

			tmpl, err4 := b.templateSvc.GetTemplateByID(ctx, notification.Template.ID)
			if err4 != nil {
				b.logger.Warn("模板ID非法",
					elog.FieldErr(err4),
					elog.Any("req", req),
					elog.Any("info", info),
					elog.Any("模板ID", notification.Template.ID))
				return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
			}

			if !tmpl.HasPublished() {
				b.logger.Warn("模板ID未发布",
					elog.Any("req", req),
					elog.Any("info", info),
					elog.Any("模板ID", notification.Template.ID))
				return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
			}
			// 设置业务ID及已发布的模版版本
			notification.BizID = bizID
			notification.Template.VersionID = tmpl.ActiveVersionID
			domainNotifications[i] = notification
		}

		// 转存MQ
		err5 := b.producer.Produce(ctx, ratelimitevt.RequestRateLimitedEvent{
			HandlerName:   notificationHandler.Name(),
			Notifications: domainNotifications,
		})
		if err5 != nil {
			// 只记录日志
			b.logger.Warn("转存限流请求失败",
				elog.FieldErr(err5),
				elog.Any("req", req),
				elog.Any("info", info))
		}

		return nil, status.Errorf(codes.ResourceExhausted, "%s", errs.ErrRateLimited)
	}
}
