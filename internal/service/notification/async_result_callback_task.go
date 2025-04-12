package notification

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"gitee.com/flycash/notification-platform/internal/service/notification/callback"
	"github.com/meoying/dlock-go"
)

type AsyncRequestResultCallbackTask struct {
	dclient     dlock.Client
	callbackSvc callback.Service
	batchSize   int64
}

func (a *AsyncRequestResultCallbackTask) Start(ctx context.Context) {
	const key = "notification_handling_async_request_result_callback"
	lj := loopjob.NewInfiniteLoop(a.dclient, a.HandleSendResult, key)
	lj.Run(ctx)
}

func (a *AsyncRequestResultCallbackTask) HandleSendResult(ctx context.Context) error {
	const minDuration = 3 * time.Second

	now := time.Now()

	err := a.callbackSvc.SendCallback(ctx, now.UnixMilli(), a.batchSize)
	if err != nil {
		return err
	}

	duration := time.Since(now)
	if duration < minDuration {
		time.Sleep(minDuration - duration)
	}

	return nil
}
