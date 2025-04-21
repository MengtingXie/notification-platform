package client

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

type RequestTypeKey string

const (
	RequestType    RequestTypeKey = "requestType"
	readWeightStr  RequestTypeKey = "read_weight"
	writeWeightStr RequestTypeKey = "write_weight"
)

type rwServiceNode struct {
	mutex                *sync.RWMutex
	conn                 balancer.SubConn
	readWeight           int32
	curReadWeight        int32
	efficientReadWeight  int32
	writeWeight          int32
	curWriteWeight       int32
	efficientWriteWeight int32
}

type RWBalancer struct {
	nodes []*rwServiceNode
}

func (r *RWBalancer) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if len(r.nodes) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	var totalWeight int32
	var selectedNode *rwServiceNode
	ctx := info.Ctx
	iswrite := r.isWrite(ctx)
	for _, node := range r.nodes {
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

type WeightBalancerBuilder struct {
	previousNodes map[balancer.SubConn]*rwServiceNode
	mu            sync.Mutex
}

func (w *WeightBalancerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Initialize previousNodes map if it's nil
	if w.previousNodes == nil {
		w.previousNodes = make(map[balancer.SubConn]*rwServiceNode)
	}

	// Create a new map for the current nodes
	newNodes := make([]*rwServiceNode, 0, len(info.ReadySCs))

	// Process all current subconnections
	for sub, subInfo := range info.ReadySCs {
		readWeight, ok := subInfo.Address.Attributes.Value(readWeightStr).(int32)
		if !ok {
			continue
		}
		writeWeight, ok := subInfo.Address.Attributes.Value(writeWeightStr).(int32)
		if !ok {
			continue
		}

		// Check if this node already exists in previous nodes
		if existingNode, found := w.previousNodes[sub]; found {
			// Keep the node with its existing state
			existingNode.conn = sub // Update connection just in case
			newNodes = append(newNodes, existingNode)
		} else {
			// Create a new node with initial weights
			newNode := &rwServiceNode{
				mutex:                &sync.RWMutex{},
				conn:                 sub,
				readWeight:           readWeight,
				curReadWeight:        readWeight,
				efficientReadWeight:  readWeight,
				writeWeight:          writeWeight,
				curWriteWeight:       writeWeight,
				efficientWriteWeight: writeWeight,
			}
			newNodes = append(newNodes, newNode)
			w.previousNodes[sub] = newNode
		}
	}

	// Create a new map for current nodes to replace the previous one
	currentNodes := make(map[balancer.SubConn]*rwServiceNode)
	for _, node := range newNodes {
		currentNodes[node.conn] = node
	}

	// Replace the previous nodes with the current ones
	w.previousNodes = currentNodes

	return &RWBalancer{
		nodes: newNodes,
	}
}
