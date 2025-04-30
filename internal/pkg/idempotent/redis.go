package idempotent

import (
	"context"

	"github.com/ecodeclub/ekit/slice"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

type BloomIdempotencyService struct {
	client     redis.Cmdable
	filterName string
	capacity   uint64  // 预期容量
	errorRate  float64 // 误判率
}

func NewBloomService(client redis.Cmdable, filterName string,
	capacity uint64, errorRate float64,
) *BloomIdempotencyService {
	return &BloomIdempotencyService{
		client:     client,
		filterName: filterName,
		capacity:   capacity,
		errorRate:  errorRate,
	}
}

func (s *BloomIdempotencyService) Exists(ctx context.Context, key string) (bool, error) {
	return s.client.BFAdd(ctx, s.filterName, key).Result()
}

func (s *BloomIdempotencyService) MExists(ctx context.Context, keys ...string) ([]bool, error) {
	if len(keys) == 0 {
		return nil, errors.New("empty keys")
	}
	// 执行批量查询
	res := s.client.BFMAdd(ctx, s.filterName, slice.Map(keys, func(_ int, src string) any {
		return src
	})...)
	return res.Result()
}
