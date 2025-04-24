package notificationv1

import (
	"errors"
	"fmt"
	"strconv"
	"time"
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
