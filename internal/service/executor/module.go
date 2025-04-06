package executor

import (
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service"
)

type (
	Service                service.ExecutorService
	Notification           = domain.Notification
	SendStrategyConfig     = domain.SendStrategyConfig
	SendStrategyType       = domain.SendStrategyType
	SendResponse           = domain.SendResponse
	BatchSendResponse      = domain.BatchSendResponse
	BatchSendAsyncResponse = domain.BatchSendAsyncResponse
	Module                 struct {
		Svc service.ExecutorService
	}
)

// 错误常量定义，导出内部服务定义的错误类型
var (
	ErrInvalidParameter        = service.ErrInvalidParameter
	ErrNotificationNotFound    = service.ErrNotificationNotFound
	ErrSendNotificationFailed  = service.ErrSendNotificationFailed
	ErrQueryNotificationFailed = service.ErrQueryNotificationFailed
)

const (
	SendStrategyImmediate  = domain.SendStrategyImmediate  // 立即发送
	SendStrategyDelayed    = domain.SendStrategyDelayed    // 延迟发送
	SendStrategyScheduled  = domain.SendStrategyScheduled  // 定时发送
	SendStrategyTimeWindow = domain.SendStrategyTimeWindow // 时间窗口发送
	SendStrategyDeadline   = domain.SendStrategyDeadline   // 截止日期发送
)
