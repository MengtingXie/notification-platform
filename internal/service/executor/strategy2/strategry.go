package strategy2

import (
	"context"
	"errors"
	"fmt"
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
)

// SendStrategy 发送策略接口
type SendStrategy interface {
	// SendImmediately 是否可立即发送
	SendImmediately(ctx context.Context) bool
	// NextSendTime 计算下次发送时间
	NextSendTime(ctx context.Context) (time.Time, error)
	// Type 策略类型
	Type() SendStrategyType
}

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

	// 根据不同参数创建不同策略
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

func (s *ImmediateStrategy) NextSendTime(_ context.Context) (time.Time, error) {
	return time.Now(), nil
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

func (s *DelayedStrategy) NextSendTime(_ context.Context) (time.Time, error) {
	return time.Now().Add(time.Duration(s.delaySeconds) * time.Second), nil
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

func (s *ScheduledStrategy) NextSendTime(_ context.Context) (time.Time, error) {
	return s.scheduledTime, nil
}

func (s *ScheduledStrategy) Type() SendStrategyType {
	return SendStrategyTypeScheduled
}

// TimeWindowStrategy 时间窗口内发送策略
type TimeWindowStrategy struct {
	windowStartTime time.Time
	windowEndTime   time.Time
}

func NewTimeWindowStrategy(startTimeMs, endTimeMs int64) *TimeWindowStrategy {
	return &TimeWindowStrategy{
		windowStartTime: time.UnixMilli(startTimeMs),
		windowEndTime:   time.UnixMilli(endTimeMs),
	}
}

func (s *TimeWindowStrategy) SendImmediately(_ context.Context) bool {
	now := time.Now()
	return now.After(s.windowStartTime) && now.Before(s.windowEndTime)
}

func (s *TimeWindowStrategy) NextSendTime(_ context.Context) (time.Time, error) {
	now := time.Now()

	// 如果当前时间早于窗口开始时间，返回窗口开始时间
	if now.Before(s.windowStartTime) {
		return s.windowStartTime, nil
	}

	// 如果当前时间在窗口内，返回当前时间
	if now.Before(s.windowEndTime) {
		return now, nil
	}

	// 如果当前时间已经超过窗口结束时间，返回错误
	return time.Time{}, ErrWindowExpired
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

func (s *DeadlineStrategy) NextSendTime(ctx context.Context) (time.Time, error) {
	t, err := s.TimeWindowStrategy.NextSendTime(ctx)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrDeadlinePassed, err)
	}
	return t, err
}

func (s *DeadlineStrategy) Type() SendStrategyType {
	return SendStrategyTypeDeadline
}
