package executor

import (
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service"
)

type (
	Service            service.ExecutorService
	Notification       = domain.Notification
	SendStrategyConfig = domain.SendStrategyConfig
	SendStrategyType   = domain.SendStrategyType
	ErrorCode          = domain.ErrorCode
	SendResponse       = domain.SendResponse
	Module             struct {
		Svc service.ExecutorService
	}
)

const (
	SendStrategyImmediate  = domain.SendStrategyImmediate  // 立即发送
	SendStrategyDelayed    = domain.SendStrategyDelayed    // 延迟发送
	SendStrategyScheduled  = domain.SendStrategyScheduled  // 定时发送
	SendStrategyTimeWindow = domain.SendStrategyTimeWindow // 时间窗口发送
	SendStrategyDeadline   = domain.SendStrategyDeadline   // 截止日期发送

	ErrorCodeUnspecified              = domain.ErrorCodeUnspecified              // 未指定错误码
	ErrorCodeInvalidParameter         = domain.ErrorCodeInvalidParameter         // 无效参数
	ErrorCodeRateLimited              = domain.ErrorCodeRateLimited              // 频率限制
	ErrorCodeTemplateNotFound         = domain.ErrorCodeTemplateNotFound         // 模板未找到
	ErrorCodeChannelDisabled          = domain.ErrorCodeChannelDisabled          // 渠道被禁用
	ErrorCodeCreateNotificationFailed = domain.ErrorCodeCreateNotificationFailed // 创建通知失败
)
