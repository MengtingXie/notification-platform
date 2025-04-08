package domain

import (
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/errs"
)

// SendStrategyType 发送策略类型
type SendStrategyType string

const (
	SendStrategyImmediate  SendStrategyType = "IMMEDIATE"   // 立即发送
	SendStrategyDelayed    SendStrategyType = "DELAYED"     // 延迟发送
	SendStrategyScheduled  SendStrategyType = "SCHEDULED"   // 定时发送
	SendStrategyTimeWindow SendStrategyType = "TIME_WINDOW" // 时间窗口发送
	SendStrategyDeadline   SendStrategyType = "DEADLINE"    // 截止日期发送
)

// SendStrategyConfig 发送策略配置
type SendStrategyConfig struct {
	Type          SendStrategyType // 发送策略类型
	Delay         time.Duration    // 延迟发送策略使用
	ScheduledTime time.Time        // 定时发送策略使用，计划发送时间
	StartTime     time.Time        // 窗口发送策略使用，开始时间（毫秒）
	EndTime       time.Time        // 窗口发送策略使用，结束时间（毫秒）
	DeadlineTime  time.Time        // 截止日期策略使用，截止日期
}

// SendTimeWindow 计算最早发送时间和最晚发送时间
func (e SendStrategyConfig) SendTimeWindow() (time.Time, time.Time) {
	switch e.Type {
	case SendStrategyImmediate:
		now := time.Now()
		const defaultEndMinute = 30
		return now, now.Add(time.Minute * defaultEndMinute)
	case SendStrategyDelayed:
		now := time.Now()
		return now, now.Add(e.Delay)
	case SendStrategyDeadline:
		now := time.Now()
		return now, e.DeadlineTime
	case SendStrategyTimeWindow:
		return e.StartTime, e.EndTime
	case SendStrategyScheduled:
		// 无法精确控制，所以允许一些误差
		const scheduledTimeTolerance = 3
		return e.ScheduledTime.Add(-time.Second * scheduledTimeTolerance), e.ScheduledTime
	default:
		// 假定一定检测过了，所以这里随便返回一个就可以
		now := time.Now()
		return now, now
	}
}

func (e SendStrategyConfig) Validate() error {
	// 校验策略相关字段
	switch e.Type {
	case SendStrategyImmediate:
		return nil
	case SendStrategyDelayed:
		if e.Delay <= 0 {
			return fmt.Errorf("%w: 延迟发送策略需要指定正数的延迟秒数", errs.ErrInvalidParameter)
		}
	case SendStrategyScheduled:
		if e.ScheduledTime.IsZero() || e.ScheduledTime.Before(time.Now()) {
			return fmt.Errorf("%w: 定时发送策略需要指定未来的发送时间", errs.ErrInvalidParameter)
		}
	case SendStrategyTimeWindow:
		if e.StartTime.IsZero() || e.StartTime.After(e.EndTime) {
			return fmt.Errorf("%w: 时间窗口发送策略需要指定有效的开始和结束时间", errs.ErrInvalidParameter)
		}
	case SendStrategyDeadline:
		if e.DeadlineTime.IsZero() || e.DeadlineTime.Before(time.Now()) {
			return fmt.Errorf("%w: 截止日期发送策略需要指定未来的发送时间", errs.ErrInvalidParameter)
		}
	}
	return nil
}

// SendResponse 发送响应
type SendResponse struct {
	NotificationID uint64     // 通知ID
	Status         SendStatus // 发送状态
	RetryCount     int8       // 重试次数
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	Results      []SendResponse // 所有结果
	TotalCount   int            // 总数
	SuccessCount int            // 成功数
}

// BatchSendAsyncResponse 批量异步发送响应
type BatchSendAsyncResponse struct {
	NotificationIDs []uint64 // 生成的通知ID列表
}
