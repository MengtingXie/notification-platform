package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/pkg/batchsize"
	"gitee.com/flycash/notification-platform/internal/pkg/mqx"
	"gitee.com/flycash/notification-platform/internal/pkg/ratelimit"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gotomicro/ego/core/elog"
)

const (
	defaultPollInterval = time.Second
)

type EventConsumer struct {
	srv      notificationsvc.SendService
	consumer mqx.Consumer

	limiter          ratelimit.Limiter
	limitedKey       string
	lookbackDuration time.Duration
	sleepDuration    time.Duration

	batchSize         int
	batchTimeout      time.Duration
	batchSizeAdjuster batchsize.Adjuster

	logger *elog.Component
}

func NewEventConsumer(
	srv notificationsvc.SendService,
	consumer *kafka.Consumer,
	limitedKey string,
	limiter ratelimit.Limiter,
	lookbackDuration time.Duration,
	sleepDuration time.Duration,
	batchSize int,
	batchTimeout time.Duration,
	batchSizeAdjuster batchsize.Adjuster,
) (*EventConsumer, error) {
	return NewEventConsumerWithTopic(
		srv,
		consumer,
		limitedKey,
		limiter,
		lookbackDuration,
		sleepDuration,
		batchSize,
		batchTimeout,
		batchSizeAdjuster,
		EventName,
	)
}

func NewEventConsumerWithTopic(
	srv notificationsvc.SendService,
	consumer *kafka.Consumer,
	limitedKey string,
	limiter ratelimit.Limiter,
	lookbackDuration time.Duration,
	sleepDuration time.Duration,
	batchSize int,
	batchTimeout time.Duration,
	batchSizeAdjuster batchsize.Adjuster,
	topic string,
) (*EventConsumer, error) {
	err := consumer.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		return nil, err
	}

	return &EventConsumer{
		srv:               srv,
		consumer:          consumer,
		limitedKey:        limitedKey,
		limiter:           limiter,
		lookbackDuration:  lookbackDuration,
		sleepDuration:     sleepDuration,
		batchSize:         batchSize,
		batchTimeout:      batchTimeout,
		batchSizeAdjuster: batchSizeAdjuster,
		logger:            elog.DefaultLogger,
	}, nil
}

func (c *EventConsumer) Start(ctx context.Context) {
	go func() {
		for {
			er := c.Consume(ctx)
			if er != nil {
				c.logger.Error("消费通知事件失败", elog.FieldErr(er))
			}
		}
	}()
}

func (c *EventConsumer) Consume(ctx context.Context) error {
	var (
		groupedBatchNotifications = make(map[int64][]domain.Notification)
		curBatchSize              = 0
		processedMessages         []*kafka.Message
	)

	batchTimer := time.NewTimer(c.batchTimeout)
	defer batchTimer.Stop()
	var evt Event

collectBatch:
	for {
		select {
		case <-ctx.Done():
			// ctx 被取消
			break collectBatch
		case <-batchTimer.C:
			// 达到时间限制，跳出循环
			break collectBatch
		default:
			// 达到批量大小
			if curBatchSize >= c.batchSize {
				break collectBatch
			}
		}

		msg, err := c.consumer.ReadMessage(c.batchTimeout)
		if err != nil {
			var kErr kafka.Error
			if errors.As(err, &kErr) && kErr.Code() == kafka.ErrTimedOut {
				// 聚合当前批次已超时
				break
			}
			return fmt.Errorf("获取消息失败: %w", err)
		}

		if err = json.Unmarshal(msg.Value, &evt); err != nil {
			c.logger.Warn("解析消息失败",
				elog.FieldErr(err),
				elog.Any("msg", msg))

			// 解析失败，直接提交这条消息，跳过通知
			// if _, cmErr := c.consumer.CommitMessage(msg); cmErr != nil {
			// 	c.logger.Warn("提交消息失败",
			// 		elog.FieldErr(cmErr),
			// 		elog.Any("msg", msg))
			// 	return cmErr
			// }

			// 解析失败，跳过本条，继续下一轮
			continue
		}

		const first = 0
		notification := evt.Notifications[first]
		if _, ok := groupedBatchNotifications[notification.BizID]; !ok {
			groupedBatchNotifications[notification.BizID] = make([]domain.Notification, 0)
		}
		groupedBatchNotifications[notification.BizID] = append(groupedBatchNotifications[notification.BizID], evt.Notifications...)
		curBatchSize += len(evt.Notifications)
		processedMessages = append(processedMessages, msg)
	}

	// 若本批次没有任何数据直接返回
	if len(groupedBatchNotifications) == 0 {
		return nil
	}

	// 限流检测
	if err := c.waitUntilRateLimitExpires(ctx); err != nil {
		return err
	}

	// 执行落库操作
	if err := c.batchSendNotificationsAsync(ctx, groupedBatchNotifications); err != nil {
		return err
	}

	// 按分区ID将消息分组，存储每个分区的最后一条消息
	lastMessages := make(map[int32]*kafka.Message)
	for _, msg := range processedMessages {
		lastMessages[msg.TopicPartition.Partition] = msg
	}
	// 只提交每个分区的最后一条消息
	for _, lastMsg := range lastMessages {
		if _, err := c.consumer.CommitMessage(lastMsg); err != nil {
			c.logger.Warn("提交消息失败",
				elog.FieldErr(err),
				elog.Any("partition", lastMsg.TopicPartition.Partition),
				elog.Any("offset", lastMsg.TopicPartition.Offset))
			return err
		}
	}

	return nil
}

func (c *EventConsumer) batchSendNotificationsAsync(ctx context.Context, groupedBatchNotifications map[int64][]domain.Notification) error {
	// 走异步批量发送逻辑落库
	start := time.Now()
	for bizID := range groupedBatchNotifications {
		// 同一个 BizID 通知在一个批次
		if _, err := c.srv.BatchSendNotificationsAsync(ctx, groupedBatchNotifications[bizID]...); err != nil {

			if errors.Is(err, errs.ErrNotificationDuplicate) {
				// 上次落库成功，但后续提交消费进度失败，此次重复消费导致的数据库层面的唯一索引冲突
				continue
			}

			// 其他类型的错误，记录日志
			c.logger.Warn("走异步批量发送逻辑落库失败",
				elog.FieldErr(err),
				elog.Int64("bizID", bizID),
				elog.Int("notifications_count", len(groupedBatchNotifications[bizID])))

			return err
		}
	}

	// 根据响应时间来计算新的batchSize
	newBatchSize, err := c.batchSizeAdjuster.Adjust(ctx, time.Since(start))
	if err == nil {
		c.batchSize = newBatchSize
	}
	return nil
}

func (c *EventConsumer) waitUntilRateLimitExpires(ctx context.Context) error {
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
			return nil
		}

		// 获取分配的分区
		partitions, err2 := c.consumer.Assignment()
		if err2 != nil {
			c.logger.Warn("获取消费者已分配的分区失败",
				elog.FieldErr(err2),
				elog.Any("partitions", partitions))
			return err2
		}

		// 发生过限流，睡眠一段时间，醒了继续判断是否被限流
		// 暂停分区消费
		err3 := c.consumer.Pause(partitions)
		if err3 != nil {
			c.logger.Warn("暂停分区失败",
				elog.FieldErr(err3),
				elog.Any("partitions", partitions))
			return err3
		}

		// 睡眠
		c.sleepAndPoll(c.sleepDuration)

		// 恢复分区消费
		err4 := c.consumer.Resume(partitions)
		if err4 != nil {
			c.logger.Warn("恢复分区失败",
				elog.FieldErr(err4),
				elog.Any("partitions", partitions))
			return err4
		}
	}
}

func (c *EventConsumer) sleepAndPoll(subTime time.Duration) {
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
