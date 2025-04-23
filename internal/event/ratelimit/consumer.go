package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	"github.com/ecodeclub/mq-api"
	"github.com/gotomicro/ego/core/elog"
)

//go:generate mockgen -source=./consumer.go -package=evtmocks -destination=../mocks/notification_server.mock.go -typed NotificationServer
type NotificationServer interface {
	notificationv1.NotificationServiceServer
	notificationv1.NotificationQueryServiceServer
}

type RequestRateLimitedEventConsumer struct {
	srv      NotificationServer
	consumer mq.Consumer

	limiter          ratelimit.Limiter
	limitedKey       string
	lookbackDuration time.Duration
	sleepDuration    time.Duration

	logger *elog.Component
}

func NewRequestLimitedEventConsumer(
	srv NotificationServer,
	q mq.MQ,
	limitedKey string,
	limiter ratelimit.Limiter,
	lookbackDuration time.Duration,
	sleepDuration time.Duration,
) (*RequestRateLimitedEventConsumer, error) {
	const groupID = "rateLimited"
	consumer, err := q.Consumer(eventName, groupID)
	if err != nil {
		return nil, err
	}
	return &RequestRateLimitedEventConsumer{
		srv:              srv,
		consumer:         consumer,
		limitedKey:       limitedKey,
		limiter:          limiter,
		lookbackDuration: lookbackDuration,
		sleepDuration:    sleepDuration,
		logger:           elog.DefaultLogger,
	}, nil
}

func (c *RequestRateLimitedEventConsumer) Start(ctx context.Context) {
	go func() {
		for {
			er := c.Consume(ctx)
			if er != nil {
				c.logger.Error("限流请求事件失败", elog.FieldErr(er))
			}
		}
	}()
}

func (c *RequestRateLimitedEventConsumer) Consume(ctx context.Context) error {
	for {
		// x分钟前到现在是否发送过限流
		limited, err1 := c.limiter.IsLimitedAfter(ctx, c.limitedKey, time.Now().Add(-c.lookbackDuration).UnixMilli())
		if err1 != nil {
			// ???
			return err1
		}

		if !limited {
			break
		}

		// 发生过限流，睡眠一段时间，醒了继续判断是否被限流
		time.Sleep(c.sleepDuration)
	}

	msg, err := c.consumer.Consume(ctx)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	var evt RequestRateLimitedEvent
	err = json.Unmarshal(msg.Value, &evt)
	if err != nil {
		c.logger.Warn("解析消息失败",
			elog.FieldErr(err),
			elog.Any("msg", msg.Value))

		return err
	}

	// 执行操作入库
	switch req := evt.Request.(type) {
	case *notificationv1.SendNotificationRequest:
		_, err = c.srv.SendNotification(ctx, req)
	case *notificationv1.SendNotificationAsyncRequest:
		_, err = c.srv.SendNotificationAsync(ctx, req)
	case *notificationv1.BatchSendNotificationsRequest:
		_, err = c.srv.BatchSendNotifications(ctx, req)
	case *notificationv1.BatchSendNotificationsAsyncRequest:
		_, err = c.srv.BatchSendNotificationsAsync(ctx, req)
	case *notificationv1.TxPrepareRequest:
		_, err = c.srv.TxPrepare(ctx, req)
	case *notificationv1.TxCommitRequest:
		_, err = c.srv.TxCommit(ctx, req)
	case *notificationv1.TxCancelRequest:
		_, err = c.srv.TxCancel(ctx, req)
	case *notificationv1.QueryNotificationRequest:
		_, err = c.srv.QueryNotification(ctx, req)
	case *notificationv1.BatchQueryNotificationsRequest:
		_, err = c.srv.BatchQueryNotifications(ctx, req)
	default:
		c.logger.Warn("未知业务消息类型",
			elog.Any("request", evt.Request),
		)
		err = nil
	}
	return err
}
