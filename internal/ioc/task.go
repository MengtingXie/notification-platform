package ioc

import (
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/notification/callback"
	"gitee.com/flycash/notification-platform/internal/service/scheduler"
)

func InitTasks(t1 *callback.AsyncRequestResultCallbackTask,
	t2 scheduler.NotificationScheduler,
	t3 *notification.SendingTimeoutTask,
	t4 *notification.TxCheckTask,
) []Task {
	return []Task{
		t1,
		t2,
		t3,
		t4,
	}
}
