package service

import (
	"context"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
)

// Channel 通知渠道
type Channel string

const (
	ChannelUnspecified Channel = "UNSPECIFIED"
	ChannelSMS         Channel = "SMS"    // 短信
	ChannelEmail       Channel = "EMAIL"  // 邮件
	ChannelInApp       Channel = "IN_APP" // 站内信
)

// SendStatus 通知发送状态
type SendStatus string

const (
	SendStatusUnspecified SendStatus = "UNSPECIFIED" // 未指定状态
	SendStatusPrepare     SendStatus = "PREPARE"     // 准备中
	SendStatusCanceled    SendStatus = "CANCELED"    // 已取消
	SendStatusPending     SendStatus = "PENDING"     // 待发送
	SendStatusSucceeded   SendStatus = "SUCCEEDED"   // 发送成功
	SendStatusFailed      SendStatus = "FAILED"      // 发送失败
)

// ErrorCode 错误代码
type ErrorCode int

const (
	ErrorCodeUnspecified                 ErrorCode = 0 // 未指定错误码
	ErrorCodeInvalidParameter            ErrorCode = 1 // 无效参数
	ErrorCodeRateLimited                 ErrorCode = 2 // 频率限制
	ErrorCodeTemplateNotFound            ErrorCode = 3 // 模板未找到
	ErrorCodeChannelDisabled             ErrorCode = 4 // 渠道被禁用
	ErrorCodeCreateNotificationFailed    ErrorCode = 5 // 创建通知失败
	ErrorCodeSendSendStrategyUnspecified           = 6
)

// SendStrategy 发送策略类型
type SendStrategy string

const (
	SendStrategyImmediate  SendStrategy = "IMMEDIATE"   // 立即发送
	SendStrategyDelayed    SendStrategy = "DELAYED"     // 延迟发送
	SendStrategyScheduled  SendStrategy = "SCHEDULED"   // 定时发送
	SendStrategyTimeWindow SendStrategy = "TIME_WINDOW" // 时间窗口发送
)

// ExecutorService 执行器
//
//go:generate mockgen -source=./types.go -destination=../mocks/executor.mock.go -package=executormocks -typed ExecutorService
type ExecutorService interface {
	// SendNotification 同步单条发送
	SendNotification(ctx context.Context, n Notification) (SendResponse, error)
	// SendNotificationAsync 异步单条发送
	SendNotificationAsync(ctx context.Context, n Notification) (SendResponse, error)
	// BatchSendNotifications 同步批量发送
	BatchSendNotifications(ctx context.Context, ns ...Notification) (BatchSendResponse, error)
	// BatchSendNotificationsAsync 异步批量发送
	BatchSendNotificationsAsync(ctx context.Context, ns ...Notification) (BatchSendAsyncResponse, error)
	// BatchQueryNotifications 同步批量查询
	BatchQueryNotifications(ctx context.Context, keys ...string) ([]SendResponse, error)
}

// Notification 通知模型
type Notification struct {
	BizID          int64             // 业务ID
	Key            string            // 业务内唯一标识
	Receiver       string            // 接收者
	Channel        Channel           // 通知渠道
	TemplateID     int64             // 模板ID
	TemplateParams map[string]string // 模板参数

	// 策略相关字段
	Strategy              SendStrategy // 发送策略类型
	DelaySeconds          int64        // 延迟秒数，用于延迟发送策略
	ScheduledTime         time.Time    // 计划发送时间，用于定时发送策略
	StartTimeMilliseconds int64        // 开始时间（毫秒），用于时间窗口策略
	EndTimeMilliseconds   int64        // 结束时间（毫秒），用于时间窗口策略
}

// ToDomainNotification 转换为领域模型
func (n Notification) ToDomainNotification(id uint64) domain.Notification {
	// 处理发送策略，默认为立即发送（当前时间 + 24小时窗口）
	now := time.Now()
	scheduledSTime := now.Unix()
	const d = 24
	scheduledETime := now.Add(d * time.Hour).Unix()

	// 构建领域模型
	return domain.Notification{
		ID:             id,
		BizID:          n.BizID,
		Key:            n.Key,
		Receiver:       n.Receiver,
		Channel:        domain.Channel(n.Channel),
		TemplateID:     n.TemplateID,
		Status:         domain.StatusPending,
		ScheduledSTime: scheduledSTime,
		ScheduledETime: scheduledETime,
	}
}

// SendResponse 发送响应
type SendResponse struct {
	NotificationID uint64     // 通知ID
	Status         SendStatus // 发送状态
	SendTime       time.Time  // 发送时间
	ErrorCode      ErrorCode  // 错误代码
	ErrorMessage   string     // 错误信息
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
