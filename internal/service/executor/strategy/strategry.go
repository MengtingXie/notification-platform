package strategy

import (
	"context"
	"errors"
	"time"
)

// 定义错误
var (
	ErrWindowExpired  = errors.New("时间窗口已过期")
	ErrDeadlinePassed = errors.New("截止时间已过")
)

// SendStrategyType 策略类型常量
type SendStrategyType string

const (
	// SendStrategyTypeImmediate  立即发送
	SendStrategyTypeImmediate SendStrategyType = "immediate"
	// SendStrategyTypeDelayed 延迟发送
	SendStrategyTypeDelayed SendStrategyType = "delayed"
	// SendStrategyTypeScheduled 定点发送
	SendStrategyTypeScheduled SendStrategyType = "scheduled"
	// SendStrategyTypeTimeWindow 时间窗口内发送
	SendStrategyTypeTimeWindow SendStrategyType = "time_window"
	// SendStrategyTypeDeadline 截止日期前发送
	SendStrategyTypeDeadline SendStrategyType = "deadline"

	// 允许的发送延迟
	allowedSendDelay = 1000
)

// SendStrategyParams 内部策略参数模型
type SendStrategyParams struct {
	// 延迟发送秒数
	DelaySeconds int64

	// 定时发送的具体时间
	ScheduledTime time.Time

	// 时间窗口起始时间(毫秒时间戳)和结束时间(毫秒时间戳)
	WindowStartTime int64
	WindowEndTime   int64

	// 截止发送时间
	DeadlineTime time.Time
}

// SendStrategy 发送策略接口
type SendStrategy interface {
	// SendImmediately 是否可立即发送
	SendImmediately(ctx context.Context) bool

	// CalculateSendTimeWindow 计算可发送的时间窗口(毫秒时间戳)
	CalculateSendTimeWindow(ctx context.Context) (start, end int64, err error)
	// Type 策略类型
	Type() SendStrategyType
}

// SendStrategyFactory 发送策略工厂
type SendStrategyFactory struct{}

func NewStrategyFactory() *SendStrategyFactory {
	return &SendStrategyFactory{}
}

// New 从内部参数模型创建策略
func (f *SendStrategyFactory) New(p *SendStrategyParams) (SendStrategy, error) {
	if p == nil {
		return NewImmediateStrategy(), nil
	}

	if p.DelaySeconds > 0 {
		return NewDelayedStrategy(p.DelaySeconds), nil
	}

	if !p.ScheduledTime.IsZero() {
		return NewScheduledStrategy(p.ScheduledTime), nil
	}

	if p.WindowStartTime > 0 && p.WindowEndTime > 0 {
		return NewTimeWindowStrategy(p.WindowStartTime, p.WindowEndTime), nil
	}

	if !p.DeadlineTime.IsZero() {
		return NewDeadlineStrategy(p.DeadlineTime), nil
	}

	return NewImmediateStrategy(), nil
}

// ImmediateStrategy 立即发送策略
type ImmediateStrategy struct{}

func NewImmediateStrategy() *ImmediateStrategy {
	return &ImmediateStrategy{}
}

func (s *ImmediateStrategy) SendImmediately(_ context.Context) bool {
	return true
}

func (s *ImmediateStrategy) CalculateSendTimeWindow(_ context.Context) (start, end int64, err error) {
	now := time.Now().UnixMilli()
	return now, now, nil
}

func (s *ImmediateStrategy) Type() SendStrategyType {
	return SendStrategyTypeImmediate
}

// DelayedStrategy 延迟发送策略
type DelayedStrategy struct {
	delaySeconds int64
}

func NewDelayedStrategy(delaySeconds int64) *DelayedStrategy {
	return &DelayedStrategy{
		delaySeconds: delaySeconds,
	}
}

func (s *DelayedStrategy) SendImmediately(_ context.Context) bool {
	return s.delaySeconds <= 0
}

func (s *DelayedStrategy) CalculateSendTimeWindow(_ context.Context) (start, end int64, err error) {
	now := time.Now()
	start = now.Add(time.Duration(s.delaySeconds) * time.Second).UnixMilli()
	end = start + allowedSendDelay // 延迟发送的窗口为1秒
	return start, end, nil
}

func (s *DelayedStrategy) Type() SendStrategyType {
	return SendStrategyTypeDelayed
}

// ScheduledStrategy 定时发送策略
type ScheduledStrategy struct {
	scheduledTime time.Time
}

func NewScheduledStrategy(scheduledTime time.Time) *ScheduledStrategy {
	return &ScheduledStrategy{
		scheduledTime: scheduledTime,
	}
}

func (s *ScheduledStrategy) SendImmediately(_ context.Context) bool {
	return time.Now().Equal(s.scheduledTime) || time.Now().After(s.scheduledTime)
}

func (s *ScheduledStrategy) CalculateSendTimeWindow(_ context.Context) (start, end int64, err error) {
	scheduledMs := s.scheduledTime.UnixMilli()
	return scheduledMs, scheduledMs + allowedSendDelay, nil
}

func (s *ScheduledStrategy) Type() SendStrategyType {
	return SendStrategyTypeScheduled
}

// TimeWindowStrategy 时间窗口内发送策略
type TimeWindowStrategy struct {
	windowStartTime int64
	windowEndTime   int64
}

func NewTimeWindowStrategy(startTimeMs, endTimeMs int64) *TimeWindowStrategy {
	return &TimeWindowStrategy{
		windowStartTime: startTimeMs,
		windowEndTime:   endTimeMs,
	}
}

func (s *TimeWindowStrategy) SendImmediately(_ context.Context) bool {
	now := time.Now().UnixMilli()
	return s.windowStartTime <= now && now <= s.windowEndTime
}

func (s *TimeWindowStrategy) CalculateSendTimeWindow(_ context.Context) (start, end int64, err error) {
	now := time.Now().UnixMilli()

	// 如果当前时间早于窗口开始时间，返回窗口开始时间
	if now < s.windowStartTime {
		return s.windowStartTime, s.windowEndTime, nil
	}

	// 如果当前时间在窗口内，返回当前时间和窗口结束时间
	if now < s.windowEndTime {
		return now, s.windowEndTime, nil
	}

	// 如果当前时间已经超过窗口结束时间，返回错误
	return 0, 0, ErrWindowExpired
}

func (s *TimeWindowStrategy) Type() SendStrategyType {
	return SendStrategyTypeTimeWindow
}

// DeadlineStrategy 截止时间前发送策略
type DeadlineStrategy struct {
	*TimeWindowStrategy
}

func NewDeadlineStrategy(deadlineTime time.Time) *DeadlineStrategy {
	return &DeadlineStrategy{
		TimeWindowStrategy: NewTimeWindowStrategy(time.Now().UnixMilli(), deadlineTime.UnixMilli()),
	}
}

func (s *DeadlineStrategy) SendImmediately(ctx context.Context) bool {
	return s.TimeWindowStrategy.SendImmediately(ctx)
}

func (s *DeadlineStrategy) CalculateSendTimeWindow(ctx context.Context) (start, end int64, err error) {
	st, ed, err := s.TimeWindowStrategy.CalculateSendTimeWindow(ctx)
	if err != nil {
		return 0, 0, ErrDeadlinePassed
	}
	return st, ed, err
}

func (s *DeadlineStrategy) Type() SendStrategyType {
	return SendStrategyTypeDeadline
}
