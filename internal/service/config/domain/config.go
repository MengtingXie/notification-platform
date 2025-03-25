package domain

import (
	"context"
	"errors"
	"time"
)

// BusinessConfig 业务配置领域对象
type BusinessConfig struct {
	ID            int64  // 业务标识
	OwnerID       int64  // 业务方ID
	OwnerType     string // 业务方类型：person-个人,organization-组织
	ChannelConfig string // 渠道配置，JSON格式
	TxnConfig     string // 事务配置，JSON格式
	RateLimit     int    // 每秒最大请求数
	Quota         string // 配额设置，JSON格式
	RetryPolicy   string // 重试策略，JSON格式
	Ctime         int64  // 创建时间
	Utime         int64  // 更新时间
}

// BusinessConfigRepository 业务配置仓库接口
type BusinessConfigRepository interface {
	// GetByID 根据ID获取单个业务配置
	GetByID(ctx context.Context, id int64) (*BusinessConfig, error)

	// GetByIDs 根据多个ID批量获取业务配置
	GetByIDs(ctx context.Context, ids []int64) ([]*BusinessConfig, error)

	// Delete 删除业务配置
	Delete(ctx context.Context, id int64) error

	// SaveNonZeroConfig 保存业务配置（新增或更新非零字段）
	SaveNonZeroConfig(ctx context.Context, config *BusinessConfig) error

	// GetByOwner 根据业务方信息获取配置
	GetByOwner(ctx context.Context, ownerID int64, ownerType string) (*BusinessConfig, error)

	// List 列出业务配置
	List(ctx context.Context, offset, limit int, filters map[string]interface{}) ([]*BusinessConfig, int64, error)
}

// BusinessConfigRepositoryError 业务配置仓库错误类型
var (
	ErrConfigNotFound     = errors.New("业务配置不存在")
	ErrConfigAlreadyExist = errors.New("业务配置已存在")
	ErrInvalidConfigData  = errors.New("无效的业务配置数据")
)

// NewBusinessConfig 创建新的业务配置
func NewBusinessConfig(ownerID int64, ownerType string) (*BusinessConfig, error) {
	// 验证业务方类型是否有效
	if ownerType != "person" && ownerType != "organization" {
		return nil, errors.New("无效的业务方类型")
	}

	// 设置默认的通道配置
	channelConfig := `{"allowed_channels":["SMS","EMAIL"],"default":"SMS"}`

	// 设置默认的事务配置
	txnConfig := `{}`

	// 设置默认的配额
	quota := `{"monthly":{"SMS":100000,"EMAIL":500000}}`

	// 设置默认的重试策略
	retryPolicy := `{"max_attempts":3,"backoff":"EXPONENTIAL"}`

	// 创建时间戳
	now := time.Now().Unix()

	return &BusinessConfig{
		OwnerID:       ownerID,
		OwnerType:     ownerType,
		ChannelConfig: channelConfig,
		TxnConfig:     txnConfig,
		RateLimit:     1000, // 默认每秒1000请求
		Quota:         quota,
		RetryPolicy:   retryPolicy,
		Ctime:         now,
		Utime:         now,
	}, nil
}
