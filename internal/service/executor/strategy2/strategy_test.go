package strategy2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试具体策略的 SendImmediately 和 NextSendTime
func TestSendStrategies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                   string
		strategy               SendStrategy
		wantCanSendImmediately bool
		wantError              bool
	}{
		{
			name:                   "Immediate Strategy",
			strategy:               NewImmediateStrategy(),
			wantCanSendImmediately: true,
			wantError:              false,
		},
		{
			name:                   "Delayed Strategy - Immediate",
			strategy:               NewDelayedStrategy(0),
			wantCanSendImmediately: true,
			wantError:              false,
		},
		{
			name:                   "Delayed Strategy - Delayed",
			strategy:               NewDelayedStrategy(60),
			wantCanSendImmediately: false,
			wantError:              false,
		},
		{
			name:                   "Scheduled Strategy - Past",
			strategy:               NewScheduledStrategy(time.Now().Add(-1 * time.Hour)),
			wantCanSendImmediately: true,
			wantError:              false,
		},
		{
			name:                   "Scheduled Strategy - Future",
			strategy:               NewScheduledStrategy(time.Now().Add(1 * time.Hour)),
			wantCanSendImmediately: false,
			wantError:              false,
		},
		{
			name: "Time Window Strategy - Inside Window",
			strategy: func() *TimeWindowStrategy {
				now := time.Now()
				return NewTimeWindowStrategy(
					now.Add(-1*time.Hour).UnixMilli(),
					now.Add(1*time.Hour).UnixMilli(),
				)
			}(),
			wantCanSendImmediately: true,
			wantError:              false,
		},
		{
			name: "Time Window Strategy - Outside Window",
			strategy: func() *TimeWindowStrategy {
				now := time.Now()
				return NewTimeWindowStrategy(
					now.Add(-2*time.Hour).UnixMilli(),
					now.Add(-1*time.Hour).UnixMilli(),
				)
			}(),
			wantCanSendImmediately: false,
			wantError:              true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.wantCanSendImmediately, tc.strategy.SendImmediately(t.Context()))

			sendTime, err := tc.strategy.NextSendTime(t.Context())
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.False(t, sendTime.IsZero())
			}
		})
	}
}
