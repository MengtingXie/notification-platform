package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqx "gitee.com/flycash/notification-platform/internal/pkg/mqx2"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gotomicro/ego/core/elog"
)

const (
	defaultPollInterval = time.Second
)

type RequestRateLimitedEventConsumer struct {
	srv      notificationsvc.SendService
	consumer mqx.Consumer

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
	err := consumer.SubscribeTopics([]string{eventName}, nil)
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

	// 不管通知的原始发送策略是什么，经过MQ转存后，一律强转为默认的截止日期前发送，等地异步任务调度并发送
	_, err = c.srv.BatchSendNotificationsAsync(ctx, evt.Notifications...)
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
