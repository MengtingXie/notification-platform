//go:build e2e

package ratelimit

import (
	"fmt"
	"testing"
	"time"

	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRedisSlidingWindowLimiter(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RedisSlidingWindowLimiterTestSuite))
}

type RedisSlidingWindowLimiterTestSuite struct {
	suite.Suite
	rdb     redis.Cmdable
	limiter *RedisSlidingWindowLimiter
}

func (s *RedisSlidingWindowLimiterTestSuite) SetupSuite() {
	s.rdb = testioc.InitRedis()
}

func (s *RedisSlidingWindowLimiterTestSuite) SetupTest() {
	t := s.T()

	// 为每个测试创建一个带有短时间窗口的限流器 使用100ms窗口以快速测试滑动功能
	s.limiter = NewRedisSlidingWindowLimiter(s.rdb, 100*time.Millisecond, 5)

	// 清理测试前缀的键，避免测试间相互影响
	ctx := t.Context()
	keys, err := s.rdb.Keys(ctx, "ratelimit:test:*").Result()
	require.NoError(t, err)

	if len(keys) > 0 {
		s.rdb.Del(ctx, keys...)
	}
}

// 生成唯一的测试键，避免测试冲突
func (s *RedisSlidingWindowLimiterTestSuite) getUniqueKey(name string) string {
	return fmt.Sprintf("test:%s:%d", name, time.Now().UnixNano())
}

// TestLimit_SingleRequest 测试单个请求不触发限流
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_SingleRequest() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("single_request")

	// 第一个请求应该不会被限制
	limited, err := s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.False(t, limited, "第一个请求不应该被限流")

	// 验证Redis中记录了请求数据
	cnt, err := s.rdb.ZCard(ctx, s.limiter.getCountKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), cnt, "应该记录一个请求")
}

// TestLimit_ExceedThreshold 测试超过阈值触发限流
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_ExceedThreshold() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("exceed_threshold")

	// 发送5个请求，都应通过
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 第6个请求应该被限流
	limited, err := s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第6个请求应该被限流")

	// 验证Redis中记录了限流事件
	eventCnt, err := s.rdb.ZCard(ctx, s.limiter.getLimitedEventsKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), eventCnt, "应该记录一个限流事件")
}

// TestLimit_DifferentKeys 测试不同key之间互不影响
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_DifferentKeys() {
	t := s.T()
	ctx := t.Context()
	key1 := s.getUniqueKey("key1")
	key2 := s.getUniqueKey("key2")

	// 填满key1的限流窗口
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key1)
		require.NoError(t, err)
		assert.False(t, limited, "填充窗口的请求不应被限流")
	}

	// key1应该被限流
	limited, err := s.limiter.Limit(ctx, key1)
	require.NoError(t, err)
	assert.True(t, limited, "key1应该被限流")

	// key2应该不受影响
	limited, err = s.limiter.Limit(ctx, key2)
	require.NoError(t, err)
	assert.False(t, limited, "key2不应该被限流")
}

// TestLimit_WindowSliding 测试窗口滑动后限流恢复
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_WindowSliding() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("window_sliding")

	// 填满限流窗口
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, "填充窗口的请求不应被限流")
	}

	// 确认已触发限流
	limited, err := s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "窗口已满应该被限流")

	// 等待窗口滑动(100ms窗口 + 额外余量)
	time.Sleep(150 * time.Millisecond)

	// 窗口滑动后应该可以再次请求
	limited, err = s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.False(t, limited, "窗口滑动后不应该被限流")
}

// TestLimit_CustomTime 测试使用自定义时间点进行限流判断
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_CustomTime() {
	t := s.T()
	// 创建一个自定义时间窗口的限流器
	// 窗口1秒，最多允许3个请求
	customLimiter := NewRedisSlidingWindowLimiter(s.rdb, 1*time.Second, 3)

	ctx := t.Context()
	key := s.getUniqueKey("custom_window")

	// 发送3个请求，都应该通过
	for i := 0; i < 3; i++ {
		limited, err := customLimiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 第4个请求应该被限流
	limited, err := customLimiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第4个请求应该被限流")

	// 验证请求记录数量
	cnt, err := s.rdb.ZCard(ctx, customLimiter.getCountKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), cnt, "应该记录三个请求")

	// 验证限流事件记录
	eventCnt, err := s.rdb.ZCard(ctx, customLimiter.getLimitedEventsKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), eventCnt, "应该记录一个限流事件")
}

// TestIsLimitedAfter_NoEvents 测试没有限流事件时的查询
func (s *RedisSlidingWindowLimiterTestSuite) TestIsLimitedAfter_NoEvents() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("no_limit_events")

	// 没有限流事件发生
	limited, err := s.limiter.IsLimitedAfter(ctx, key, time.Now().Add(-1*time.Hour).UnixMilli())
	require.NoError(t, err)
	assert.False(t, limited, "没有限流事件时应返回false")
}

// TestIsLimitedAfter_WithEvents 测试有限流事件时的查询
func (s *RedisSlidingWindowLimiterTestSuite) TestIsLimitedAfter_WithEvents() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("with_limit_events")

	// 获取当前时间作为测试起点
	startTime := time.Now().UnixMilli()

	// 填满限流窗口并触发限流
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 验证第6个请求被限流
	limited, err := s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第6个请求应该被限流")

	// 查询从起点时间后是否有限流事件
	limited, err = s.limiter.IsLimitedAfter(ctx, key, startTime)
	require.NoError(t, err)
	assert.True(t, limited, "应该检测到限流事件")

	// 查询从未来时间点后是否有限流事件
	futureTime := time.Now().Add(1 * time.Second).UnixMilli()
	limited, err = s.limiter.IsLimitedAfter(ctx, key, futureTime)
	require.NoError(t, err)
	assert.False(t, limited, "未来时间点后不应有限流事件")
}

// TestIsLimitedAfter_TimeRangeAccuracy 测试时间范围查询的准确性
func (s *RedisSlidingWindowLimiterTestSuite) TestIsLimitedAfter_TimeRangeAccuracy() {
	t := s.T()
	ctx := t.Context()
	key := s.getUniqueKey("time_range")

	// 阶段1: 记录时间点
	timeBeforeLimiting := time.Now().UnixMilli()

	// 触发限流
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 验证第6个请求被限流
	limited, err := s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第6个请求应该被限流")

	// 阶段2: 记录限流后的时间点
	timeAfterLimiting := time.Now().UnixMilli()

	// 使用限流前的时间点查询，应该可以检测到限流
	limited, err = s.limiter.IsLimitedAfter(ctx, key, timeBeforeLimiting)
	require.NoError(t, err)
	assert.True(t, limited, "从限流前的时间点查询应该检测到限流")

	// 等待窗口滑动
	time.Sleep(150 * time.Millisecond)

	// 阶段3: 在限流事件之后再触发一次新的限流
	for i := 0; i < 5; i++ {
		limited, err := s.limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("窗口滑动后第%d个请求不应被限流", i+1))
	}

	// 验证再次触发限流
	limited, err = s.limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "应该再次触发限流")

	// 使用第一次限流后的时间点查询，应该只检测到第二次限流
	limited, err = s.limiter.IsLimitedAfter(ctx, key, timeAfterLimiting)
	require.NoError(t, err)
	assert.True(t, limited, "从限流后的时间点查询应该检测到第二次限流")
}
