//go:build e2e

package idempotent

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestAllImplementations 测试所有可用的幂等性服务实现
func TestAllImplementations(t *testing.T) {
	t.Parallel()

	// 检查Redis是否可用
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	t.Cleanup(func() {
		client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server is not available, skipping all implementation tests")
		return
	}

	// 以下测试需要外部依赖，默认跳过，但可通过命令行参数启用
	// 例如: go test -v -run=TestAllImplementations/RedisImplementation ./internal/pkg/idempotent
	t.Run("RedisImplementation", func(t *testing.T) {
		TestRedisImplementation(t)
	})
	t.Run("BloomFilterImplementation", func(t *testing.T) {
		TestBloomFilterImplementation(t)
	})
}
