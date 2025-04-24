package failover

import (
	"context"
	"errors"
	"sync/atomic"
)

type Selector struct {
	nodes []Node
	idx   atomic.Int64
}

func (s *Selector) Invoke(ctx context.Context, req any) (any, error) {
	// 在内部解决 failover 的问题
	start := s.idx.Add(1)
	for i := 0; i < len(s.nodes); i++ {
		idx := (start + int64(i)) % int64(len(s.nodes))
		resp, err := s.nodes[idx].Send(ctx, req)
		if err == nil {
			return resp, nil
		}
	}
	return nil, errors.New("全部节点遍历了一遍,都失败了")
}

func (s *Selector) InvokeV1(ctx context.Context, req any) (any, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		// 在内部解决 failover 的问题
		// ABCD
		idx := s.idx.Add(1) % int64(len(s.nodes))
		resp, err := s.nodes[idx].Send(ctx, req)
		if err == nil {
			return resp, nil
		}
	}
	return nil, errors.New("全部节点遍历了一遍,都失败了")
}

// Node 代表一个节点
type Node struct{}

//func (n Node) Health() bool {
//	// 做健康检测，心跳之类的
//}

func (n Node) Send(_ context.Context, _ any) (any, error) {
	panic("implement me")
}
