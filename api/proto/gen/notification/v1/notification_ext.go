package notificationv1

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
)

func (x *Notification) FindReceivers() []string {
	receivers := x.Receivers
	if x.Receiver != "" {
		receivers = append(receivers, x.Receiver)
	}
	return receivers
}

// CustomValidate 你加方法，可以做很多事情
func (x *Notification) CustomValidate() error {
	switch val := x.Strategy.StrategyType.(type) {
	case *SendStrategy_Delayed:
		// 延迟时间超过 1 小时，你就返回错误
		if time.Duration(val.Delayed.DelaySeconds)*time.Second > time.Hour*24 {
			return errors.New("延迟太久了")
		}
	}
	return nil
}

// ReceiversAsUid 比如说站内信之类，receivers 其实是 uid
func (x *Notification) ReceiversAsUid() ([]int64, error) {
	receivers := x.FindReceivers()
	result := make([]int64, 0, len(receivers))
	for _, r := range receivers {
		val, err := strconv.ParseInt(r, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("必须是数字 %w", err)
		}
		result = append(result, val)
	}
	return result, nil
}

type NotificationHandler interface {
	Name() string
	GetNotifications() []*Notification
}

func (x *SendNotificationRequest) Name() string {
	return "SendNotification"
}

func (x *SendNotificationRequest) GetNotifications() []*Notification {
	return []*Notification{x.GetNotification()}
}

func (x *SendNotificationAsyncRequest) Name() string {
	return "SendNotificationAsync"
}

func (x *SendNotificationAsyncRequest) GetNotifications() []*Notification {
	return []*Notification{x.GetNotification()}
}

func (x *BatchSendNotificationsRequest) Name() string {
	return "BatchSendNotifications"
}

func (x *BatchSendNotificationsAsyncRequest) Name() string {
	return "BatchSendNotificationsAsync"
}

func (x *Notification) ToDomainNotification() (domain.Notification, error) {
	if x == nil {
		return domain.Notification{}, fmt.Errorf("%w: 通知信息不能为空", errs.ErrInvalidParameter)
	}

	tid, err := strconv.ParseInt(x.TemplateId, 10, 64)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%w: 模板ID: %s", errs.ErrInvalidParameter, x.TemplateId)
	}

	channel, err := x.getDomainChannel()
	if err != nil {
		return domain.Notification{}, err
	}

	return domain.Notification{
		Key:       x.Key,
		Receivers: x.FindReceivers(),
		Channel:   channel,
		Template: domain.Template{
			ID:     tid,
			Params: x.TemplateParams,
		},
		SendStrategyConfig: x.getDomainSendStrategyConfig(),
	}, nil
}

func (x *Notification) getDomainChannel() (domain.Channel, error) {
	switch x.Channel {
	case Channel_SMS:
		return domain.ChannelSMS, nil
	case Channel_EMAIL:
		return domain.ChannelEmail, nil
	case Channel_IN_APP:
		return domain.ChannelInApp, nil
	default:
		return "", fmt.Errorf("%w", errs.ErrUnknownChannel)
	}
}

func (x *Notification) getDomainSendStrategyConfig() domain.SendStrategyConfig {
	// 构建发送策略
	sendStrategyType := domain.SendStrategyImmediate // 默认为立即发送
	var delaySeconds int64
	var scheduledTime time.Time
	var startTimeMilliseconds int64
	var endTimeMilliseconds int64
	var deadlineTime time.Time

	// 处理发送策略
	if x.Strategy != nil {
		switch s := x.Strategy.StrategyType.(type) {
		case *SendStrategy_Immediate:
			sendStrategyType = domain.SendStrategyImmediate
		case *SendStrategy_Delayed:
			if s.Delayed != nil && s.Delayed.DelaySeconds > 0 {
				sendStrategyType = domain.SendStrategyDelayed
				delaySeconds = s.Delayed.DelaySeconds
			}
		case *SendStrategy_Scheduled:
			if s.Scheduled != nil && s.Scheduled.SendTime != nil {
				sendStrategyType = domain.SendStrategyScheduled
				scheduledTime = s.Scheduled.SendTime.AsTime()
			}
		case *SendStrategy_TimeWindow:
			if s.TimeWindow != nil {
				sendStrategyType = domain.SendStrategyTimeWindow
				startTimeMilliseconds = s.TimeWindow.StartTimeMilliseconds
				endTimeMilliseconds = s.TimeWindow.EndTimeMilliseconds
			}
		case *SendStrategy_Deadline:
			if s.Deadline != nil && s.Deadline.Deadline != nil {
				sendStrategyType = domain.SendStrategyDeadline
				deadlineTime = s.Deadline.Deadline.AsTime()
			}
		}
	}
	return domain.SendStrategyConfig{
		Type:          sendStrategyType,
		Delay:         time.Duration(delaySeconds) * time.Second,
		ScheduledTime: scheduledTime,
		StartTime:     time.Unix(startTimeMilliseconds, 0),
		EndTime:       time.Unix(endTimeMilliseconds, 0),
		DeadlineTime:  deadlineTime,
	}
}
