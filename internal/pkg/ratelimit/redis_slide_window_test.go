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
	t.Skip()
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
	keys, err := s.rdb.Keys(t.Context(), "ratelimit:*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 0)
}

func (s *RedisSlidingWindowLimiterTestSuite) newLimiter() *RedisSlidingWindowLimiter {
	return NewRedisSlidingWindowLimiter(s.rdb, 100*time.Millisecond, 5)
}

// 生成唯一的测试键，避免测试冲突
func (s *RedisSlidingWindowLimiterTestSuite) getUniqueKey(name string) string {
	return fmt.Sprintf("test:%s:%d", name, time.Now().UnixNano())
}

// TestLimit_SingleRequest 测试单个请求不触发限流
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_SingleRequest() {
	t := s.T()
	t.Skip()
	ctx := t.Context()
	key := s.getUniqueKey("single_request")

	// 第一个请求应该不会被限制
	limiter := s.newLimiter()

	limited, err := limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.False(t, limited, "第一个请求不应该被限流")

	// 验证Redis中记录了请求数据
	cnt, err := s.rdb.ZCard(ctx, limiter.getCountKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), cnt, "应该记录一个请求")

	s.rdb.Del(ctx, limiter.getCountKey(key))
}

// TestLimit_ExceedThreshold 测试超过阈值触发限流
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_ExceedThreshold() {
	t := s.T()
	t.Skip()
	ctx := t.Context()
	key := s.getUniqueKey("exceed_threshold")

	// 发送5个请求，都应通过
	limiter := s.newLimiter()
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 第6个请求应该被限流
	limited, err := limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第6个请求应该被限流")

	// 验证Redis中记录了限流事件
	exists, err := s.rdb.Exists(ctx, limiter.getLimitedEventKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "应该存在限流记录")

	s.rdb.Del(ctx, limiter.getCountKey(key), limiter.getLimitedEventKey(key))
}

// TestLimit_DifferentKeys 测试不同key之间互不影响
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_DifferentKeys() {
	t := s.T()

	ctx := t.Context()
	key1 := s.getUniqueKey("key1")
	key2 := s.getUniqueKey("key2")

	// 填满key1的限流窗口
	limiter := s.newLimiter()
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key1)
		require.NoError(t, err)
		assert.False(t, limited, "填充窗口的请求不应被限流")
	}

	// key1应该被限流
	limited, err := limiter.Limit(ctx, key1)
	require.NoError(t, err)
	assert.True(t, limited, "key1应该被限流")

	// key2应该不受影响
	limited, err = limiter.Limit(ctx, key2)
	require.NoError(t, err)
	assert.False(t, limited, "key2不应该被限流")

	s.rdb.Del(ctx, limiter.getCountKey(key1), limiter.getLimitedEventKey(key1))
	s.rdb.Del(ctx, limiter.getCountKey(key2), limiter.getLimitedEventKey(key2))
}

// TestLimit_WindowSliding 测试窗口滑动后限流恢复
func (s *RedisSlidingWindowLimiterTestSuite) TestLimit_WindowSliding() {
	t := s.T()

	ctx := t.Context()
	key := s.getUniqueKey("window_sliding")

	// 填满限流窗口
	limiter := s.newLimiter()
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, "填充窗口的请求不应被限流")
	}

	// 确认已触发限流
	limited, err := limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "窗口已满应该被限流")

	// 等待窗口滑动(100ms窗口 + 额外余量)
	time.Sleep(150 * time.Millisecond)

	// 窗口滑动后应该可以再次请求
	limited, err = limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.False(t, limited, "窗口滑动后不应该被限流")

	s.rdb.Del(ctx, limiter.getCountKey(key), limiter.getLimitedEventKey(key))
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
	exists, err := s.rdb.Exists(ctx, customLimiter.getLimitedEventKey(key)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "应该存在限流记录")

	s.rdb.Del(ctx, customLimiter.getCountKey(key), customLimiter.getLimitedEventKey(key))
}

// TestLastLimitTime_NoEvents 测试没有限流事件时的查询
func (s *RedisSlidingWindowLimiterTestSuite) TestLastLimitTime_NoEvents() {
	t := s.T()

	ctx := t.Context()
	key := s.getUniqueKey("no_limit_events")

	// 没有限流事件发生
	limiter := s.newLimiter()
	limitTime, err := limiter.LastLimitTime(ctx, key)
	require.NoError(t, err)
	assert.True(t, limitTime.IsZero(), "没有限流事件时应返回零值时间")

	s.rdb.Del(ctx, limiter.getCountKey(key), limiter.getLimitedEventKey(key))
}

// TestLastLimitTime_WithEvents 测试有限流事件时的查询
func (s *RedisSlidingWindowLimiterTestSuite) TestLastLimitTime_WithEvents() {
	t := s.T()

	ctx := t.Context()
	key := s.getUniqueKey("with_limit_events")

	// 获取当前时间作为测试起点
	startTime := time.Now()

	// 填满限流窗口并触发限流
	limiter := s.newLimiter()
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第%d个请求不应该被限流", i+1))
	}

	// 触发限流
	limited, err := limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第6个请求应该被限流")

	// 查询最后限流时间
	limitTime, err := limiter.LastLimitTime(ctx, key)
	require.NoError(t, err)
	assert.False(t, limitTime.IsZero(), "应该有限流时间")

	// 验证时间在合理范围内
	assert.True(t, limitTime.After(startTime) || limitTime.Equal(startTime), "限流时间应该在测试开始之后或相等")
	assert.True(t, limitTime.Before(time.Now().Add(1*time.Second)), "限流时间应该接近当前时间")

	s.rdb.Del(ctx, limiter.getCountKey(key), limiter.getLimitedEventKey(key))
}

// TestLastLimitTime_MultipleEvents 测试多次限流情况下获取最新的限流时间
func (s *RedisSlidingWindowLimiterTestSuite) TestLastLimitTime_MultipleEvents() {
	t := s.T()

	ctx := t.Context()
	key := s.getUniqueKey("multiple_limit_events")

	// 第一次限流
	limiter := s.newLimiter()
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第一轮第%d个请求不应该被限流", i+1))
	}

	// 触发第一次限流
	limited, err := limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第一轮第6个请求应该被限流")

	firstLimitTime, err := limiter.LastLimitTime(ctx, key)
	require.NoError(t, err)
	assert.False(t, firstLimitTime.IsZero())

	// 等待窗口滑动
	time.Sleep(time.Second)

	// 第二次限流
	for i := 0; i < 5; i++ {
		limited, err := limiter.Limit(ctx, key)
		require.NoError(t, err)
		assert.False(t, limited, fmt.Sprintf("第二轮第%d个请求不应该被限流", i+1))
	}

	// 触发第二次限流
	limited, err = limiter.Limit(ctx, key)
	require.NoError(t, err)
	assert.True(t, limited, "第二轮第6个请求应该被限流")

	secondLimitTime, err := limiter.LastLimitTime(ctx, key)
	require.NoError(t, err)

	// 验证第二次限流时间晚于第一次
	assert.True(t, secondLimitTime.After(firstLimitTime), "第二次限流应晚于第一次")
	s.rdb.Del(ctx, limiter.getCountKey(key), limiter.getLimitedEventKey(key))
}
