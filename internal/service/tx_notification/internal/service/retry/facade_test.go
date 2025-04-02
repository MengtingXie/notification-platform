package retry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFacade(t *testing.T) {
	t.Parallel()
	// 创建测试用的构建器映射
	builderMap := map[string]Builder{
		"normal": &NormalBuilder{},
	}
	// 添加一个mock实现，用于验证时间戳计算准确性
	mockRetryStrategy := StrategyFunc(func(req Req) (int64, bool) {
		if req.CheckTimes >= 3 {
			return 0, false
		}
		// 返回固定的时间计算方式，便于测试断言
		return time.Now().UnixMilli() + 30000, true
	})
	mockBuilder := &MockBuilder{strategy: mockRetryStrategy}
	builderMap["mock"] = mockBuilder

	// 创建FacadeBuilder
	facadeBuilder := NewFacadeBuilder(builderMap)

	// 测试场景
	testCases := []struct {
		name         string
		configJSON   string
		checkTimes   int
		expectError  bool
		expectNextOK bool
	}{
		{
			name:         "正常重试策略-首次尝试",
			configJSON:   `{"type":"normal","maxRetryTimes":3,"interval":10}`,
			checkTimes:   0,
			expectError:  false,
			expectNextOK: true,
		},
		{
			name:         "正常重试策略-第二次尝试",
			configJSON:   `{"type":"normal","maxRetryTimes":3,"interval":10}`,
			checkTimes:   1,
			expectError:  false,
			expectNextOK: true,
		},
		{
			name:         "正常重试策略-达到最大重试次数",
			configJSON:   `{"type":"normal","maxRetryTimes":3,"interval":10}`,
			checkTimes:   3,
			expectError:  false,
			expectNextOK: false,
		},
		{
			name:         "Mock策略-验证时间戳",
			configJSON:   `{"type":"mock"}`,
			checkTimes:   0,
			expectError:  false,
			expectNextOK: true,
		},
		{
			name:        "未知重试策略类型",
			configJSON:  `{"type":"unknown","maxRetryTimes":3,"interval":10}`,
			expectError: true,
		},
		{
			name:        "无效的JSON格式",
			configJSON:  `{invalid-json}`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// 构建重试策略
			strategy, err := facadeBuilder.Build(tc.configJSON)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, strategy)

			// 记录调用时间，用于验证时间戳计算
			beforeCall := time.Now().UnixMilli()

			// 测试NextTime方法
			nextTime, ok := strategy.NextTime(Req{CheckTimes: tc.checkTimes})
			assert.Equal(t, tc.expectNextOK, ok)

			if ok {
				// 确保nextTime是有效的未来时间戳（毫秒级）
				assert.Greater(t, nextTime, int64(0))

				if tc.name == "Mock策略-验证时间戳" {
					// 对于mock策略，我们知道固定加了30秒
					expectedTime := beforeCall + 30000
					// 允许100ms的误差范围
					assert.InDelta(t, expectedTime, nextTime, 100, "时间戳应该是当前时间+30秒")
				} else {
					// 对于正常策略，解析配置并验证时间戳计算
					var normalCfg normalConfig
					err := json.Unmarshal([]byte(tc.configJSON), &normalCfg)
					require.NoError(t, err)

					// Normal策略中，NextTime实现是 time.Now().UnixMilli() + cfg.MaxRetryTimes*1000
					expectedTime := beforeCall + int64(normalCfg.MaxRetryTimes*1000)
					// 允许100ms的误差范围
					assert.InDelta(t, expectedTime, nextTime, 100, "时间戳计算不准确")
				}
			}
		})
	}

	// 测试空的构建器映射
	t.Run("空构建器映射", func(t *testing.T) {
		emptyBuilder := NewFacadeBuilder(map[string]Builder{})
		_, err := emptyBuilder.Build(`{"type":"normal"}`)
		require.Error(t, err)
	})
}

// MockBuilder 用于测试的构建器
type MockBuilder struct {
	strategy Strategy
}

func (m *MockBuilder) Build(_ string) (Strategy, error) {
	return m.strategy, nil
}
