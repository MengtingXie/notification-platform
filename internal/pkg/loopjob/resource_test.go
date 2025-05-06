package loopjob

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceSemaphore_AcquireRelease(t *testing.T) {
	t.Parallel()
	r := NewResourceSemaphore(2)

	ctx := t.Context()
	err := r.Acquire(ctx)
	assert.NoError(t, err)

	err = r.Release(ctx)
	assert.NoError(t, err)
}

func TestResourceSemaphore_ConcurrentOverLimit(t *testing.T) {
	t.Parallel()
	r := NewResourceSemaphore(3)
	var wg sync.WaitGroup
	errCh := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- r.Acquire(t.Context())
		}()
	}

	wg.Wait()
	close(errCh)

	errorCount := 0
	for err := range errCh {
		if err != nil {
			errorCount++
		}
	}
	assert.Equal(t, 2, errorCount)
}

func TestResourceSemaphore_CounterCorrectness(t *testing.T) {
	t.Parallel()
	r := NewResourceSemaphore(3)
	var wg sync.WaitGroup

	// 获取3次
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, r.Acquire(t.Context()))
		}()
	}
	wg.Wait()

	// 释放2次
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, r.Release(t.Context()))
		}()
	}
	wg.Wait()

	// 再次获取1次应该成功
	assert.NoError(t, r.Acquire(t.Context()))
}
