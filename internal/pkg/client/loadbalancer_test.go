package client

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

func TestRWBalancer_isWrite(t *testing.T) {
	balancer := &RWBalancer{}
	t.Parallel()
	// 测试用例
	testCases := []struct {
		name     string
		context  context.Context
		expected bool
	}{
		{
			name:     "空值",
			context:  t.Context(),
			expected: false,
		},
		{
			name:     "非整数值",
			context:  context.WithValue(t.Context(), RequestType, "not an int"),
			expected: false,
		},
		{
			name:     "整数 0 (读操作)",
			context:  context.WithValue(t.Context(), RequestType, 0),
			expected: false,
		},
		{
			name:     "整数 1 (写操作)",
			context:  context.WithValue(t.Context(), RequestType, 1),
			expected: true,
		},
		{
			name:     "整数 2 (非写操作)",
			context:  context.WithValue(t.Context(), RequestType, 2),
			expected: false,
		},
	}

	// 运行测试用例
	for idx := range testCases {
		tc := testCases[idx]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := balancer.isWrite(tc.context)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// mockSubConn 是 balancer.SubConn 的测试实现
type mockSubConn struct {
	balancer.SubConn
	name string
}

func (m *mockSubConn) Name() string {
	return m.name
}

// createTestNode 创建一个具有指定权重和连接的测试 rwServiceNode
func createTestNode(name string, readWeight, writeWeight int32) *rwServiceNode {
	return &rwServiceNode{
		conn:                 &mockSubConn{name: name},
		mutex:                &sync.RWMutex{},
		readWeight:           readWeight,
		curReadWeight:        readWeight,
		efficientReadWeight:  readWeight,
		writeWeight:          writeWeight,
		curWriteWeight:       writeWeight,
		efficientWriteWeight: writeWeight,
	}
}

func TestRWBalancer_Pick(t *testing.T) {
	t.Parallel()
	// 设置具有不同权重的测试节点
	nodes := []*rwServiceNode{
		createTestNode("weight-4", 1, 4), // 低读权重，高写权重
		createTestNode("weight-3", 2, 3), // 中低读权重，中高写权重
		createTestNode("weight-2", 3, 2), // 中高读权重，中低写权重
		createTestNode("weight-1", 4, 1), // 高读权重，低写权重
	}

	b := &RWBalancer{nodes: nodes}

	// 定义测试负载均衡行为的操作
	// 每个操作指定：
	// - requestType: 0 表示读，1 表示写
	// - expectedName: 预期选择的节点名称
	// - err: 在 DoneInfo 中返回的错误，模拟成功或失败的操作
	operations := []struct {
		requestType  int    // 0 = 读，1 = 写
		expectedName string // 预期选择的节点名称
		err          error  // 在 DoneInfo 中返回的错误
		description  string // 此操作的描述
	}{
		{0, "weight-1", nil, "读操作应该选择最高读权重的节点"},
		{1, "weight-4", context.DeadlineExceeded, "写操作应该选择最高写权重的节点，带超时错误"},
		{0, "weight-2", nil, "第二次读操作应该由于权重调整选择次高读权重的节点"},
		{1, "weight-3", io.EOF, "第二次写操作应该选择次高写权重的节点，带 EOF 错误"},
		{0, "weight-3", nil, "第三次读操作选择已调整的读权重节点"},
		{1, "weight-2", nil, "第三次写操作选择已调整的写权重节点"},
		{0, "weight-1", nil, "第四次读操作选择已调整的读权重节点"},
		{1, "weight-4", nil, "第四次写操作选择已调整的写权重节点"},
		{0, "weight-2", nil, "第五次读操作基于权重继续轮询"},
		{1, "weight-3", nil, "第五次写操作基于权重继续轮询"},
	}

	// 顺序执行所有操作以验证负载均衡
	for i := range operations {
		op := operations[i]
		t.Run(op.description, func(t *testing.T) {
			t.Parallel()
			// 创建具有适当请求类型的上下文
			ctx := context.WithValue(t.Context(), RequestType, op.requestType)

			// 执行 Pick 方法
			pickRes, err := b.Pick(balancer.PickInfo{Ctx: ctx})

			// 验证在选择过程中没有发生错误
			require.NoError(t, err, "操作 %d 在 Pick 期间不应失败", i)

			// 验证选择了正确的节点
			selectedConn, ok := pickRes.SubConn.(*mockSubConn)
			require.True(t, ok, "SubConn 应该是 mockSubConn 类型")
			assert.Equal(t, op.expectedName, selectedConn.Name(), "操作 %d 选择了错误的节点", i)

			// 调用 Done 来模拟操作完成，带有指定的错误
			pickRes.Done(balancer.DoneInfo{Err: op.err})
		})
	}
}

// TestWeightBalancerBuilder_Build 测试 WeightBalancerBuilder 的功能
func TestWeightBalancerBuilder_Build(t *testing.T) {
	t.Parallel()
	// 创建一个新的构建器
	builder := &WeightBalancerBuilder{
		previousNodes: make(map[balancer.SubConn]*rwServiceNode),
	}

	// 设置模拟连接
	mockConn1 := &mockSubConn{name: "conn1"}
	mockConn2 := &mockSubConn{name: "conn2"}

	// 创建可以添加属性的地址
	addr1 := resolver.Address{
		Addr: "addr1",
	}
	addr2 := resolver.Address{
		Addr: "addr2",
	}

	// 使用 attributes 包创建并添加属性
	addr1.Attributes = attributes.New(readWeightStr, int32(10))
	addr1.Attributes = addr1.Attributes.WithValue(writeWeightStr, int32(20))

	addr2.Attributes = attributes.New(readWeightStr, int32(30))
	addr2.Attributes = addr2.Attributes.WithValue(writeWeightStr, int32(40))

	// 创建带有正确属性的 SubConnInfo 对象
	readySCs := map[balancer.SubConn]base.SubConnInfo{
		mockConn1: {
			Address: addr1,
		},
		mockConn2: {
			Address: addr2,
		},
	}

	// 构建选择器
	picker := builder.Build(base.PickerBuildInfo{ReadySCs: readySCs})

	// 验证选择器是 RWBalancer
	rwPicker, ok := picker.(*RWBalancer)
	require.True(t, ok, "选择器应该是 RWBalancer 类型")

	// 验证节点已正确创建
	require.Equal(t, 2, len(rwPicker.nodes), "RWBalancer 应该有 2 个节点")

	// 验证节点具有正确的权重，并映射回正确的连接
	nodeMap := make(map[string]*rwServiceNode)
	for _, node := range rwPicker.nodes {
		conn, ok := node.conn.(*mockSubConn)
		require.True(t, ok, "连接应该是 mockSubConn 类型")
		nodeMap[conn.Name()] = node
	}

	// 验证 conn1 的权重
	conn1Node, found := nodeMap["conn1"]
	require.True(t, found, "应该找到 conn1 节点")
	assert.Equal(t, int32(10), conn1Node.readWeight, "conn1 应该有正确的读权重")
	assert.Equal(t, int32(20), conn1Node.writeWeight, "conn1 应该有正确的写权重")

	// 验证 conn2 的权重
	conn2Node, found := nodeMap["conn2"]
	require.True(t, found, "应该找到 conn2 节点")
	assert.Equal(t, int32(30), conn2Node.readWeight, "conn2 应该有正确的读权重")
	assert.Equal(t, int32(40), conn2Node.writeWeight, "conn2 应该有正确的写权重")

	// 测试连接状态保持
	// 构建第二次选择器，应该保留先前的节点状态
	conn1Node.efficientReadWeight = 15 // 模拟一些权重变化
	conn1Node.efficientWriteWeight = 25

	picker = builder.Build(base.PickerBuildInfo{ReadySCs: readySCs})
	rwPicker, ok = picker.(*RWBalancer)
	require.True(t, ok, "选择器应该是 RWBalancer 类型")

	// 重新构建节点映射
	nodeMap = make(map[string]*rwServiceNode)
	for _, node := range rwPicker.nodes {
		conn, ok := node.conn.(*mockSubConn)
		require.True(t, ok, "连接应该是 mockSubConn 类型")
		nodeMap[conn.Name()] = node
	}

	// 验证状态已保留
	conn1Node, found = nodeMap["conn1"]
	require.True(t, found, "应该找到 conn1 节点")
	assert.Equal(t, int32(15), conn1Node.efficientReadWeight, "conn1 应该保留有效的读权重")
	assert.Equal(t, int32(25), conn1Node.efficientWriteWeight, "conn1 应该保留有效的写权重")
}
