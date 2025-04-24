package hash

import (
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestHashNoCollision(t *testing.T) {
	// 初始化随机数生成器
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 定义测试规模：1000个组合
	testSize := 1000

	// 哈希结果映射，用于检测冲突
	hashResults := make(map[int64]struct{}, testSize)
	// 存储测试输入，用于在发现冲突时输出详细信息
	inputs := make([]struct {
		bizId int64
		key   string
	}, testSize)

	// 生成1000个不同的测试用例
	for i := 0; i < testSize; i++ {
		// 生成随机bizId (1-10000范围内)
		bizId := r.Int63n(10000) + 1

		// 生成随机key (10-30个字符)
		keyLength := r.Intn(20) + 10
		key := generateRandomString(r, keyLength)

		// 存储测试输入
		inputs[i] = struct {
			bizId int64
			key   string
		}{bizId, key}

		// 计算哈希值
		hashValue := Hash(bizId, key)

		// 检查是否存在冲突
		if _, exists := hashResults[hashValue]; exists {
			// 发现冲突，找出是哪两个输入产生了相同的哈希值
			for j := 0; j < i; j++ {
				prevHashValue := Hash(inputs[j].bizId, inputs[j].key)
				if prevHashValue == hashValue {
					t.Fatalf("哈希冲突: \n"+
						"输入1: bizId=%d, key=%s \n"+
						"输入2: bizId=%d, key=%s \n"+
						"相同的哈希值: %d",
						inputs[j].bizId, inputs[j].key,
						bizId, key,
						hashValue)
				}
			}
		}

		// 记录哈希值
		hashResults[hashValue] = struct{}{}
	}

	// 检查哈希结果数量是否等于测试用例数量
	if len(hashResults) != testSize {
		t.Errorf("预期生成 %d 个不同的哈希值，实际生成 %d 个", testSize, len(hashResults))
	} else {
		t.Logf("成功测试 %d 个不同的输入组合，未发现哈希冲突", testSize)
	}
}

// 生成指定长度的随机字符串
func generateRandomString(r *rand.Rand, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = charset[r.Intn(len(charset))]
	}
	return string(result)
}

// 测试哈希算法的分布均匀性
func TestHashDistribution(t *testing.T) {
	// 可选的额外测试：检查哈希分布
	// 这个测试不是强制要求的，但可以帮助验证哈希函数的质量

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	testSize := 10000
	bucketCount := 100
	buckets := make([]int, bucketCount)

	// 生成大量哈希值
	for i := 0; i < testSize; i++ {
		bizId := r.Int63n(10000) + 1
		key := "test" + strconv.Itoa(i)

		// 计算哈希值并放入对应的桶
		hashValue := Hash(bizId, key)
		bucketIndex := int(hashValue % int64(bucketCount))
		if bucketIndex < 0 {
			bucketIndex += bucketCount // 处理负数哈希值
		}
		buckets[bucketIndex]++
	}

	// 计算理论上每个桶的平均值和允许的偏差
	expectedPerBucket := float64(testSize) / float64(bucketCount)
	maxDeviation := 0.3 * expectedPerBucket // 允许30%的偏差

	// 检查分布是否均匀
	for i, count := range buckets {
		deviation := float64(count) - expectedPerBucket
		if deviation < 0 {
			deviation = -deviation
		}

		if deviation > maxDeviation {
			t.Logf("桶 %d 的值数量 (%d) 偏离预期 (%.2f) 超过允许范围", i, count, expectedPerBucket)
		}
	}

	// 输出一些分布统计信息
	min, max, avg := buckets[0], buckets[0], float64(0)
	for _, count := range buckets {
		if count < min {
			min = count
		}
		if count > max {
			max = count
		}
		avg += float64(count)
	}
	avg /= float64(bucketCount)

	t.Logf("哈希分布统计: 最小=%d, 最大=%d, 平均=%.2f, 理论平均=%.2f",
		min, max, avg, expectedPerBucket)
}
