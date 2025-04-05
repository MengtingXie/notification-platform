package domain

import (
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification"
)

// ErrorCode 错误代码
type ErrorCode int

const (
	ErrorCodeUnspecified              ErrorCode = 0 // 未指定错误码
	ErrorCodeInvalidParameter         ErrorCode = 1 // 无效参数
	ErrorCodeRateLimited              ErrorCode = 2 // 频率限制
	ErrorCodeTemplateNotFound         ErrorCode = 3 // 模板未找到
	ErrorCodeChannelDisabled          ErrorCode = 4 // 渠道被禁用
	ErrorCodeCreateNotificationFailed ErrorCode = 5 // 创建通知失败
)

// SendStrategyType 发送策略类型
type SendStrategyType string

const (
	SendStrategyImmediate  SendStrategyType = "IMMEDIATE"   // 立即发送
	SendStrategyDelayed    SendStrategyType = "DELAYED"     // 延迟发送
	SendStrategyScheduled  SendStrategyType = "SCHEDULED"   // 定时发送
	SendStrategyTimeWindow SendStrategyType = "TIME_WINDOW" // 时间窗口发送
)

// SendStrategyConfig 发送策略配置
type SendStrategyConfig struct {
	Type                  SendStrategyType // 发送策略类型
	DelaySeconds          int64            // 延迟发送策略使用，延迟秒数
	ScheduledTime         time.Time        // 定时发送策略使用，计划发送时间
	StartTimeMilliseconds int64            // 窗口发送策略使用，开始时间（毫秒）
	EndTimeMilliseconds   int64            // 窗口发送策略使用，结束时间（毫秒）
}

// Notification 通知模型
type Notification struct {
	Notification       notification.Notification
	SendStrategyConfig SendStrategyConfig
}

// SendResponse 发送响应
type SendResponse struct {
	NotificationID uint64                  // 通知ID
	Status         notification.SendStatus // 发送状态
	SendTime       time.Time               // 发送时间
	RetryCount     int8                    // 重试次数
	ErrorCode      ErrorCode               // 错误代码
	ErrorMessage   string                  // 错误信息
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	Results      []SendResponse // 所有结果
	TotalCount   int            // 总数
	SuccessCount int            // 成功数
}

// BatchSendAsyncResponse 批量异步发送响应
type BatchSendAsyncResponse struct {
	NotificationIDs []uint64  // 生成的通知ID列表
	ErrorCode       ErrorCode // 错误代码
	ErrorMessage    string    // 错误信息
}
