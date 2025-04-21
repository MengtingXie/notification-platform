package loadbalancer

import (
	"context"
	"math/bits"
	"sync"
	"sync/atomic"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
)

const (
	defaultNumberLen   = 64  // Number of bits in uint64
	defaultFailPercent = 0.1 // Default threshold percentage for failures

	// Bit manipulation constants
	bitsPerUint64      = 64
	bitsPerUint64Shift = 6    // log2(64), used for division by 64
	bitMask            = 63   // 2^6 - 1, used for modulo 64
	initialHealth      = true // Initial health status
)

type mprovider struct {
	provider.HealthAwareProvider
	healthy       *atomic.Bool
	ringBuffer    []uint64 // 比特环（滑动窗口存储）
	reqCount      uint64   // 请求数量
	bufferLen     int      // 滑动窗口长度
	bitCnt        uint64   // 比特位总数
	failThreshold int
	mu            *sync.RWMutex
}

func (s *mprovider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	res, err := s.HealthAwareProvider.Send(ctx, notification)
	if err != nil {
		s.markFail()
		v := s.getFailed()
		if v > s.failThreshold {
			s.healthy.Store(false)
		}
	} else {
		s.markSuccess()
	}
	return res, err
}

func newMprovider(pro provider.HealthAwareProvider, bufferLen int) mprovider {
	health := &atomic.Bool{}
	health.Store(initialHealth)
	return mprovider{
		HealthAwareProvider: pro,
		healthy:             health,
		bufferLen:           bufferLen,
		ringBuffer:          make([]uint64, bufferLen),
		bitCnt:              uint64(defaultNumberLen) * uint64(bufferLen),
		mu:                  &sync.RWMutex{},
		failThreshold:       int(float64(bufferLen) * defaultFailPercent),
	}
}

func (s *mprovider) markSuccess() {
	count := atomic.AddUint64(&s.reqCount, 1)
	count %= s.bitCnt
	// 对2^x进行取模或者整除运算时可以用位运算代替除法和取模
	// count / 64 可以转换成 count >> 6。 位运算会更高效。
	idx := count >> bitsPerUint64Shift
	// count % 64 可以转换成 count & 63
	bitPos := count & bitMask
	old := atomic.LoadUint64(&s.ringBuffer[idx])
	atomic.StoreUint64(&s.ringBuffer[idx], old&^(uint64(1)<<bitPos))
}

func (s *mprovider) markFail() {
	count := atomic.AddUint64(&s.reqCount, 1)
	count %= s.bitCnt
	idx := count >> bitsPerUint64Shift
	bitPos := count & bitMask
	old := atomic.LoadUint64(&s.ringBuffer[idx])
	// (uint64(1)<<bitPos) 将目标位设置为1
	atomic.StoreUint64(&s.ringBuffer[idx], old|(uint64(1)<<bitPos))
}

func (s *mprovider) getFailed() int {
	var failCount int
	for i := 0; i < len(s.ringBuffer); i++ {
		v := atomic.LoadUint64(&s.ringBuffer[i])
		failCount += bits.OnesCount64(v)
	}
	return failCount
}

func (s *mprovider) markHealthy() {
	s.healthy.Store(true)
}
