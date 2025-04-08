package executor

import (
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification"
)

type (
	Service                = notification.ExecutorService
	Notification           = domain.Notification
	SendStrategyConfig     = domain.SendStrategyConfig
	SendStrategyType       = domain.SendStrategyType
	SendResponse           = domain.SendResponse
	BatchSendResponse      = domain.BatchSendResponse
	BatchSendAsyncResponse = domain.BatchSendAsyncResponse
	Module                 struct {
		Svc Service
	}
)

// 错误常量定义，导出内部服务定义的错误类型
var (
	ErrInvalidParameter        = ErrInvalidParameter
	ErrNotificationNotFound    = ErrNotificationNotFound
	ErrSendNotificationFailed  = ErrSendNotificationFailed
	ErrQueryNotificationFailed = ErrQueryNotificationFailed
)

const (
	SendStrategyImmediate  = domain.SendStrategyImmediate  // 立即发送
	SendStrategyDelayed    = domain.SendStrategyDelayed    // 延迟发送
	SendStrategyScheduled  = domain.SendStrategyScheduled  // 定时发送
	SendStrategyTimeWindow = domain.SendStrategyTimeWindow // 时间窗口发送
	SendStrategyDeadline   = domain.SendStrategyDeadline   // 截止日期发送
)
