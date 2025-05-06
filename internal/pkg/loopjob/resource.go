package loopjob

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

// 信号量，控制抢表的最大信号量
type ResourceSemaphore interface {
	Acquire(ctx context.Context) error
	Release(ctx context.Context) error
}

var ErrExceedLimit = errors.New("抢表超出限制")

type resourceSemaphore struct {
	maxCount int
	curCount int
	mu       *sync.RWMutex
}

func (r *resourceSemaphore) Acquire(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.curCount >= r.maxCount {
		return ErrExceedLimit
	}
	r.curCount++
	return nil
}

func (r *resourceSemaphore) Release(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.curCount--
	return nil
}

func NewResourceSemaphore(maxCount int) ResourceSemaphore {
	return &resourceSemaphore{
		maxCount: maxCount,
		mu:       &sync.RWMutex{},
		curCount: 0,
	}
}
