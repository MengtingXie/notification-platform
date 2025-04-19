package client

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/ecodeclub/ekit/slice"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

const (
	RequestType    = "requestType"
	readWeightStr  = "read_weight"
	writeWeightStr = "write_weight"
	groupStr       = "group"
)

type groupKey struct{}

type rwServiceNode struct {
	mutex                *sync.RWMutex
	conn                 balancer.SubConn
	readWeight           int32
	curReadWeight        int32
	efficientReadWeight  int32
	writeWeight          int32
	curWriteWeight       int32
	efficientWriteWeight int32
	group                string
}

type RWBalancer struct {
	nodes []*rwServiceNode
}

func (r *RWBalancer) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if len(r.nodes) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// 过滤出候选节点
	candidates := slice.FilterMap(r.nodes, func(_ int, src *rwServiceNode) (*rwServiceNode, bool) {
		return src, r.getGroup(info.Ctx) == src.group
	})

	var totalWeight int32
	var selectedNode *rwServiceNode
	ctx := info.Ctx

	iswrite := r.isWrite(ctx)
	for _, node := range candidates {
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

	if selectedNode == nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	selectedNode.mutex.Lock()
	if r.isWrite(ctx) {
		selectedNode.curWriteWeight -= totalWeight
	} else {
		selectedNode.curReadWeight -= totalWeight
	}
	selectedNode.mutex.Unlock()
	return balancer.PickResult{
		SubConn: selectedNode.conn,
		Done: func(info balancer.DoneInfo) {
			selectedNode.mutex.Lock()
			defer selectedNode.mutex.Unlock()
			isDecrementError := info.Err != nil && (errors.Is(info.Err, context.DeadlineExceeded) || errors.Is(info.Err, io.EOF))
			if r.isWrite(ctx) {
				if isDecrementError && selectedNode.efficientWriteWeight > 0 {
					selectedNode.efficientWriteWeight--
				} else if info.Err == nil {
					selectedNode.efficientWriteWeight++
				}
			} else {
				if isDecrementError && selectedNode.efficientReadWeight > 0 {
					selectedNode.efficientReadWeight--
				} else if info.Err == nil {
					selectedNode.efficientReadWeight++
				}
			}
		},
	}, nil
}

func (r *RWBalancer) isWrite(ctx context.Context) bool {
	val := ctx.Value(RequestType)
	if val == nil {
		return false
	}
	vv, ok := val.(int)
	if !ok {
		return false
	}
	return vv == 1
}

func (r *RWBalancer) getGroup(ctx context.Context) string {
	val := ctx.Value(groupKey{})
	if val == nil {
		return ""
	}
	vv, ok := val.(string)
	if !ok {
		return ""
	}
	return vv
}

type WeightBalancerBuilder struct{}

func (w *WeightBalancerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	nodes := make([]*rwServiceNode, 0, len(info.ReadySCs))
	for sub, subInfo := range info.ReadySCs {

		readWeight, ok := subInfo.Address.Attributes.Value(readWeightStr).(int32)
		if !ok {
			continue
		}
		writeWeight, ok := subInfo.Address.Attributes.Value(writeWeightStr).(int32)
		if !ok {
			continue
		}

		group, ok := subInfo.Address.Attributes.Value(groupStr).(string)
		if !ok {
			continue
		}

		nodes = append(nodes, &rwServiceNode{
			mutex:                &sync.RWMutex{},
			conn:                 sub,
			readWeight:           readWeight,
			curReadWeight:        readWeight,
			efficientReadWeight:  readWeight,
			writeWeight:          writeWeight,
			curWriteWeight:       writeWeight,
			efficientWriteWeight: writeWeight,
			group:                group,
		})
	}

	return &RWBalancer{
		nodes: nodes,
	}
}

func WithGroup(ctx context.Context, group string) context.Context {
	return context.WithValue(ctx, groupKey{}, group)
}
