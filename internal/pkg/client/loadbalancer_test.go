package client

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
)

// mockSubConn 实现了一个用于测试的最小版本的 balancer.SubConn 接口
type mockSubConn struct {
	name string
}

func (m *mockSubConn) UpdateAddresses([]resolver.Address) {}
func (m *mockSubConn) Connect()                           {}
func (m *mockSubConn) String() string                     { return m.name }

func TestRWBalancer_isWrite(t *testing.T) {
	balancer := &RWBalancer{}

	// 测试用例
	testCases := []struct {
		name     string
		context  context.Context
		expected bool
	}{
		{
			name:     "空值",
			context:  context.Background(),
			expected: false,
		},
		{
			name:     "非整数值",
			context:  context.WithValue(context.Background(), RequestType, "not an int"),
			expected: false,
		},
		{
			name:     "整数 0 (读操作)",
			context:  context.WithValue(context.Background(), RequestType, 0),
			expected: false,
		},
		{
			name:     "整数 1 (写操作)",
			context:  context.WithValue(context.Background(), RequestType, 1),
			expected: true,
		},
		{
			name:     "整数 2 (非写操作)",
			context:  context.WithValue(context.Background(), RequestType, 2),
			expected: false,
		},
	}

	// 运行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := balancer.isWrite(tc.context)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestRWBalancerPickLogic 测试 RWBalancer 的 Pick 方法的核心逻辑
func TestRWBalancerPickLogic(t *testing.T) {
	// 创建一个简单的结构体来表示我们的测试节点
	type testRWServiceNode struct {
		name                 string
		mutex                *sync.RWMutex
		readWeight           int32
		curReadWeight        int32
		efficientReadWeight  int32
		writeWeight          int32
		curWriteWeight       int32
		efficientWriteWeight int32
	}

	// 使用不同的权重初始化测试节点
	nodes := []*testRWServiceNode{
		{
			name:                 "weight-4",
			mutex:                &sync.RWMutex{},
			readWeight:           1,
			curReadWeight:        1,
			efficientReadWeight:  1,
			writeWeight:          4,
			curWriteWeight:       4,
			efficientWriteWeight: 4,
		},
		{
			name:                 "weight-3",
			mutex:                &sync.RWMutex{},
			readWeight:           2,
			curReadWeight:        2,
			efficientReadWeight:  2,
			writeWeight:          3,
			curWriteWeight:       3,
			efficientWriteWeight: 3,
		},
		{
			name:                 "weight-2",
			mutex:                &sync.RWMutex{},
			readWeight:           3,
			curReadWeight:        3,
			efficientReadWeight:  3,
			writeWeight:          2,
			curWriteWeight:       2,
			efficientWriteWeight: 2,
		},
		{
			name:                 "weight-1",
			mutex:                &sync.RWMutex{},
			readWeight:           4,
			curReadWeight:        4,
			efficientReadWeight:  4,
			writeWeight:          1,
			curWriteWeight:       1,
			efficientWriteWeight: 1,
		},
	}

	// 定义要测试的操作
	operations := []struct {
		requestType  int
		expectedName string
		err          error
		description  string
	}{
		// 基于静态权重的初始操作
		{0, "weight-1", nil, "读操作 - 最高读权重 (4)"},
		{1, "weight-4", context.DeadlineExceeded, "写操作 - 最高写权重 (4)，有错误"},

		// 继续进行更多操作以测试动态调整
		{0, "weight-2", nil, "读操作 - 第二高读权重 (3)"},
		{1, "weight-3", io.EOF, "写操作 - 第二高写权重 (3)，有错误"},
		{0, "weight-3", nil, "读操作 - 第三高读权重 (2)"},
		{1, "weight-2", nil, "写操作 - 第三高写权重 (2)，成功"},
	}

	// 跟踪选择序列的映射
	readSelections := make([]string, 0)
	writeSelections := make([]string, 0)

	// 执行操作
	for i, op := range operations {
		t.Logf("操作 %d: %s", i, op.description)

		// 确定这是否是写操作
		iswrite := op.requestType == 1

		// 模拟 Pick 逻辑
		var totalWeight int32
		var selectedNode *testRWServiceNode

		for _, node := range nodes {
			node.mutex.Lock()
			if iswrite {
				totalWeight += node.efficientWriteWeight
				node.curWriteWeight += node.efficientWriteWeight
				if selectedNode == nil || selectedNode.curWriteWeight < node.curWriteWeight {
					selectedNode = node
				}
			} else {
				totalWeight += node.efficientReadWeight
				node.curReadWeight += node.efficientReadWeight
				if selectedNode == nil || selectedNode.curReadWeight < node.curReadWeight {
					selectedNode = node
				}
			}
			node.mutex.Unlock()
		}

		require.NotNil(t, selectedNode, "操作 %d 应该选择一个节点", i)
		assert.Equal(t, op.expectedName, selectedNode.name, "操作 %d: %s", i, op.description)

		// 记录选择
		if iswrite {
			writeSelections = append(writeSelections, selectedNode.name)
		} else {
			readSelections = append(readSelections, selectedNode.name)
		}

		// 选择后调整当前权重
		selectedNode.mutex.Lock()
		if iswrite {
			selectedNode.curWriteWeight -= totalWeight
		} else {
			selectedNode.curReadWeight -= totalWeight
		}
		selectedNode.mutex.Unlock()

		// 应用完成回调逻辑
		selectedNode.mutex.Lock()
		isDecrementError := op.err != nil && (errors.Is(op.err, context.DeadlineExceeded) || errors.Is(op.err, io.EOF))
		if iswrite {
			if isDecrementError && selectedNode.efficientWriteWeight > 0 {
				selectedNode.efficientWriteWeight--
				t.Logf("由于错误，将 %s 的写权重降低到 %d", selectedNode.name, selectedNode.efficientWriteWeight)
			} else if op.err == nil {
				selectedNode.efficientWriteWeight++
				t.Logf("由于成功，将 %s 的写权重提高到 %d", selectedNode.name, selectedNode.efficientWriteWeight)
			}
		} else {
			if isDecrementError && selectedNode.efficientReadWeight > 0 {
				selectedNode.efficientReadWeight--
				t.Logf("由于错误，将 %s 的读权重降低到 %d", selectedNode.name, selectedNode.efficientReadWeight)
			} else if op.err == nil {
				selectedNode.efficientReadWeight++
				t.Logf("由于成功，将 %s 的读权重提高到 %d", selectedNode.name, selectedNode.efficientReadWeight)
			}
		}
		selectedNode.mutex.Unlock()
	}

	// 验证操作导致的预期选择结果
	assert.Equal(t, []string{"weight-1", "weight-2", "weight-3"}, readSelections, "读选择序列不正确")
	assert.Equal(t, []string{"weight-4", "weight-3", "weight-2"}, writeSelections, "写选择序列不正确")

	// 验证权重的最终状态
	t.Run("最终权重状态", func(t *testing.T) {
		// Node weight-4 (读权重 1, 写权重 4)
		assert.Equal(t, int32(1), nodes[0].efficientReadWeight, "weight-4 最终读权重不正确")
		assert.Equal(t, int32(3), nodes[0].efficientWriteWeight, "weight-4 最终写权重不正确")

		// Node weight-3 (读权重 2, 写权重 3)
		assert.Equal(t, int32(3), nodes[1].efficientReadWeight, "weight-3 最终读权重不正确")
		assert.Equal(t, int32(2), nodes[1].efficientWriteWeight, "weight-3 最终写权重不正确")

		// Node weight-2 (读权重 3, 写权重 2)
		assert.Equal(t, int32(4), nodes[2].efficientReadWeight, "weight-2 最终读权重不正确")
		assert.Equal(t, int32(3), nodes[2].efficientWriteWeight, "weight-2 最终写权重不正确")

		// Node weight-1 (读权重 4, 写权重 1)
		assert.Equal(t, int32(5), nodes[3].efficientReadWeight, "weight-1 最终读权重不正确")
		assert.Equal(t, int32(1), nodes[3].efficientWriteWeight, "weight-1 最终写权重不正确")
	})
}
