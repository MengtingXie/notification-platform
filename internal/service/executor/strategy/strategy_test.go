package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendStrategyFactory(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		params         *SendStrategyParams
		expectedType   SendStrategyType
		expectNilError bool
	}{
		{
			name:           "Immediate Strategy",
			params:         nil,
			expectedType:   SendStrategyTypeImmediate,
			expectNilError: true,
		},
		{
			name:           "Immediate Strategy",
			params:         &SendStrategyParams{},
			expectedType:   SendStrategyTypeImmediate,
			expectNilError: true,
		},
		{
			name: "Delayed Strategy",
			params: &SendStrategyParams{
				DelaySeconds: 60,
			},
			expectedType:   SendStrategyTypeDelayed,
			expectNilError: true,
		},
		{
			name: "Scheduled Strategy",
			params: &SendStrategyParams{
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedType:   SendStrategyTypeScheduled,
			expectNilError: true,
		},
		{
			name: "Time Window Strategy",
			params: &SendStrategyParams{
				WindowStartTime: time.Now().Add(1 * time.Hour).UnixMilli(),
				WindowEndTime:   time.Now().Add(2 * time.Hour).UnixMilli(),
			},
			expectedType:   SendStrategyTypeTimeWindow,
			expectNilError: true,
		},
		{
			name: "Deadline Strategy",
			params: &SendStrategyParams{
				DeadlineTime: time.Now().Add(1 * time.Hour),
			},
			expectedType:   SendStrategyTypeDeadline,
			expectNilError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			strategy, err := NewStrategyFactory().New(tc.params)
			if !tc.expectNilError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedType, strategy.Type())
		})
	}
}

func TestSendStrategies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		createStrategy func() SendStrategy

		assertImmediately    bool
		assertSendTimeWindow func(t *testing.T, start, end int64, err error)
	}{
		{
			name: "Immediate Strategy",
			createStrategy: func() SendStrategy {
				return NewImmediateStrategy()
			},
			assertImmediately: true,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				now := time.Now().UnixMilli()
				assert.InDelta(t, now, start, 1000) // 误差在1秒内
				assert.Equal(t, start, end)
			},
		},
		{
			name: "Delayed Strategy - Not Immediate",
			createStrategy: func() SendStrategy {
				return NewDelayedStrategy(60)
			},
			assertImmediately: false,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				expectedStart := time.Now().Add(60 * time.Second).UnixMilli()
				expectedEnd := expectedStart + allowedSendDelay

				assert.InDelta(t, expectedStart, start, 1000)
				assert.InDelta(t, expectedEnd, end, 1000)
			},
		},
		{
			name: "Delayed Strategy - Immediate",
			createStrategy: func() SendStrategy {
				return NewDelayedStrategy(0)
			},
			assertImmediately: true,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				now := time.Now().UnixMilli()
				assert.InDelta(t, now, start, 1000)
				assert.InDelta(t, now+allowedSendDelay, end, 1000)
			},
		},
		{
			name: "Scheduled Strategy - Past",
			createStrategy: func() SendStrategy {
				return NewScheduledStrategy(time.Now().Add(-1 * time.Hour))
			},
			assertImmediately: true,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				scheduledTime := time.Now().Add(-1 * time.Hour).UnixMilli()
				assert.Equal(t, scheduledTime, start)
				assert.Equal(t, scheduledTime+allowedSendDelay, end)
			},
		},
		{
			name: "Scheduled Strategy - Future",
			createStrategy: func() SendStrategy {
				return NewScheduledStrategy(time.Now().Add(1 * time.Hour))
			},
			assertImmediately: false,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				scheduledTime := time.Now().Add(1 * time.Hour).UnixMilli()
				assert.Equal(t, scheduledTime, start)
				assert.Equal(t, scheduledTime+allowedSendDelay, end)
			},
		},
		{
			name: "Time Window Strategy - Before Window",
			createStrategy: func() SendStrategy {
				now := time.Now()
				return NewTimeWindowStrategy(
					now.Add(1*time.Hour).UnixMilli(),
					now.Add(2*time.Hour).UnixMilli(),
				)
			},
			assertImmediately: false,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				windowStart := time.Now().Add(1 * time.Hour).UnixMilli()
				windowEnd := time.Now().Add(2 * time.Hour).UnixMilli()

				assert.Equal(t, windowStart, start)
				assert.Equal(t, windowEnd, end)
			},
		},
		{
			name: "Time Window Strategy - Within Window",
			createStrategy: func() SendStrategy {
				now := time.Now()
				return NewTimeWindowStrategy(
					now.Add(-1*time.Hour).UnixMilli(),
					now.Add(1*time.Hour).UnixMilli(),
				)
			},
			assertImmediately: true,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				windowStart := time.Now().Add(-1 * time.Hour).UnixMilli()
				windowEnd := time.Now().Add(1 * time.Hour).UnixMilli()

				assert.True(t, start >= windowStart)
				assert.True(t, start < windowEnd)
				assert.Equal(t, windowEnd, end)
			},
		},
		{
			name: "Time Window Strategy - Expired",
			createStrategy: func() SendStrategy {
				now := time.Now()
				return NewTimeWindowStrategy(
					now.Add(-2*time.Hour).UnixMilli(),
					now.Add(-1*time.Hour).UnixMilli(),
				)
			},
			assertImmediately: false,
			assertSendTimeWindow: func(t *testing.T, _, _ int64, err error) {
				t.Helper()
				assert.Error(t, err)
				assert.Equal(t, ErrWindowExpired, err)
			},
		},
		{
			name: "Deadline Strategy - Before Deadline",
			createStrategy: func() SendStrategy {
				return NewDeadlineStrategy(time.Now().Add(1 * time.Hour))
			},
			assertImmediately: true,
			assertSendTimeWindow: func(t *testing.T, start, end int64, err error) {
				t.Helper()
				require.NoError(t, err)
				now := time.Now().UnixMilli()
				deadline := time.Now().Add(1 * time.Hour).UnixMilli()

				assert.True(t, start >= now)
				assert.Equal(t, deadline, end)
			},
		},
		{
			name: "Deadline Strategy - Passed",
			createStrategy: func() SendStrategy {
				return NewDeadlineStrategy(time.Now().Add(-1 * time.Hour))
			},
			assertImmediately: false,
			assertSendTimeWindow: func(t *testing.T, _, _ int64, err error) {
				t.Helper()
				assert.Error(t, err)
				assert.Equal(t, ErrDeadlinePassed, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			strategy := tc.createStrategy()

			// 测试 SendImmediately 方法
			assert.Equal(t, tc.assertImmediately, strategy.SendImmediately(t.Context()),
				"SendImmediately 方法返回值不符合预期")

			// 测试 CalculateSendTimeWindow 方法
			start, end, err := strategy.CalculateSendTimeWindow(t.Context())
			tc.assertSendTimeWindow(t, start, end, err)
		})
	}
}
