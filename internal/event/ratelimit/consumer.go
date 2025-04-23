package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gotomicro/ego/core/elog"
)

const (
	defaultPollInterval = time.Second
)

//go:generate mockgen -source=./consumer.go -package=evtmocks -destination=../mocks/kafka_consumer.mock.go -typed KafkaConsumer
type KafkaConsumer interface {
	ReadMessage(timeout time.Duration) (*kafka.Message, error)
	Pause(partitions []kafka.TopicPartition) (err error)
	Resume(partitions []kafka.TopicPartition) (err error)
	Poll(timeoutMs int) (event kafka.Event)
	CommitMessage(m *kafka.Message) ([]kafka.TopicPartition, error)
}

type RequestRateLimitedEventConsumer struct {
	srv      notificationsvc.SendService
	consumer KafkaConsumer

	limiter          ratelimit.Limiter
	limitedKey       string
	lookbackDuration time.Duration
	sleepDuration    time.Duration

	logger *elog.Component
}

func NewRequestLimitedEventConsumer(
	srv notificationsvc.SendService,
	consumer *kafka.Consumer,
	limitedKey string,
	limiter ratelimit.Limiter,
	lookbackDuration time.Duration,
	sleepDuration time.Duration,
) (*RequestRateLimitedEventConsumer, error) {
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
				c.logger.Error("消费限流请求事件失败", elog.FieldErr(er))
			}
		}
	}()
}

func (c *RequestRateLimitedEventConsumer) Consume(ctx context.Context) error {
	msg, err := c.consumer.ReadMessage(-1)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	for {
		// 是否发送过限流
		lastLimitTime, err1 := c.limiter.LastLimitTime(ctx, c.limitedKey)
		if err1 != nil {
			c.logger.Warn("获取限流状态失败",
				elog.FieldErr(err1),
				elog.Any("limitedKey", c.limitedKey))
			return err1
		}

		// 未发生限流，或者最近一次发生限流的时间不在预期时间段内
		if lastLimitTime.IsZero() || time.Since(lastLimitTime) > c.lookbackDuration {
			break
		}

		// 发生过限流，睡眠一段时间，醒了继续判断是否被限流
		// 暂停分区消费
		err2 := c.consumer.Pause([]kafka.TopicPartition{msg.TopicPartition})
		if err2 != nil {
			c.logger.Warn("暂停分区失败",
				elog.FieldErr(err2),
				elog.Any("msg", msg))
			return err2
		}

		// 睡眠
		c.sleepAndPoll(c.sleepDuration)

		// 恢复分区消费
		err3 := c.consumer.Resume([]kafka.TopicPartition{msg.TopicPartition})
		if err3 != nil {
			c.logger.Warn("恢复分区失败",
				elog.FieldErr(err3),
				elog.Any("msg", msg))
			return err3
		}
	}

	var evt RequestRateLimitedEvent
	err = json.Unmarshal(msg.Value, &evt)
	if err != nil {
		c.logger.Warn("解析消息失败",
			elog.FieldErr(err),
			elog.Any("msg", msg))
		return err
	}

	// 执行操作入库
	err = c.handleEvent(ctx, evt)
	if err != nil {
		c.logger.Warn("处理限流请求事件失败",
			elog.FieldErr(err),
			elog.Any("evt", evt))
	}

	// 消费完成，提交消费进度
	_, err = c.consumer.CommitMessage(msg)
	if err != nil {
		c.logger.Warn("提交消息失败",
			elog.FieldErr(err),
			elog.Any("msg", msg))
		return err
	}
	return nil
}

func (c *RequestRateLimitedEventConsumer) handleEvent(ctx context.Context, evt RequestRateLimitedEvent) error {
	const first = 0
	var err error
	switch evt.HandlerName {
	case "SendNotification":
		_, err = c.srv.SendNotification(ctx, evt.Notifications[first])
	case "SendNotificationAsync":
		_, err = c.srv.SendNotificationAsync(ctx, evt.Notifications[first])
	case "BatchSendNotifications":
		_, err = c.srv.BatchSendNotifications(ctx, evt.Notifications...)
	case "BatchSendNotificationsAsync":
		_, err = c.srv.BatchSendNotificationsAsync(ctx, evt.Notifications...)
	default:
		c.logger.Warn("未知业务消息类型",
			elog.Any("request", evt.Notifications),
		)
		err = nil
	}
	return err
}

func (c *RequestRateLimitedEventConsumer) sleepAndPoll(subTime time.Duration) {
	const defaultPollDuration = 100
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()
	timer := time.NewTimer(subTime)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return
		case <-ticker.C:
			c.consumer.Poll(defaultPollDuration)
		}
	}
}
