package repository

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/service/config/domain"
	daopkg "gitee.com/flycash/notification-platform/internal/service/config/repository/dao"
	"github.com/ego-component/egorm"
)

type BusinessConfigRepository interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	// SaveNonZeroConfig 保存非零字段
	SaveNonZeroConfig(ctx context.Context, config domain.BusinessConfig) error
	// GetByOwner 根据业务方信息获取配置
	GetByOwner(ctx context.Context, ownerID int64, ownerType string) (domain.BusinessConfig, error)
	// List 列出业务配置
	List(ctx context.Context, offset, limit int, filters map[string]interface{}) ([]domain.BusinessConfig, int64, error)
}

type businessConfigRepository struct {
	configDao daopkg.BusinessConfigDAO
}

// NewBusinessConfigRepository 创建业务配置仓库实例
func NewBusinessConfigRepository(configDao daopkg.BusinessConfigDAO) BusinessConfigRepository {
	return &businessConfigRepository{
		configDao: configDao,
	}
}

// GetByIDs 根据多个ID批量获取业务配置
func (b *businessConfigRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]domain.BusinessConfig, error) {
	result := make(map[int64]domain.BusinessConfig)

	// 循环查询每个ID的配置（实际使用中可以优化为批量查询）
	for _, id := range ids {
		config, err := b.GetByID(ctx, id)
		if err != nil {
			// 如果是未找到记录的错误，则跳过
			if err == egorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		result[id] = config
	}

	return result, nil
}

// GetByID 根据ID获取业务配置
func (b *businessConfigRepository) GetByID(ctx context.Context, id int64) (domain.BusinessConfig, error) {
	// 从数据库获取配置
	daoConfig, err := b.configDao.GetByID(ctx, id)
	if err != nil {
		return domain.BusinessConfig{}, err
	}

	// 将DAO对象转换为领域对象
	return domain.BusinessConfig{
		ID:            daoConfig.ID,
		OwnerID:       daoConfig.OwnerID,
		OwnerType:     daoConfig.OwnerType,
		ChannelConfig: string(daoConfig.ChannelConfig),
		TxnConfig:     string(daoConfig.TxnConfig),
		RateLimit:     daoConfig.RateLimit,
		Quota:         string(daoConfig.Quota),
		RetryPolicy:   string(daoConfig.RetryPolicy),
		Ctime:         daoConfig.Ctime,
		Utime:         daoConfig.Utime,
	}, nil
}

// Delete 删除业务配置
func (b *businessConfigRepository) Delete(ctx context.Context, id int64) error {
	// 直接调用DAO层删除方法
	return b.configDao.Delete(ctx, id)
}

// SaveNonZeroConfig 保存业务配置（仅保存非零字段）
func (b *businessConfigRepository) SaveNonZeroConfig(ctx context.Context, config domain.BusinessConfig) error {
	// 将领域对象转换为DAO对象
	daoConfig := daopkg.BusinessConfig{
		ID:        config.ID,
		OwnerID:   config.OwnerID,
		OwnerType: config.OwnerType,
		RateLimit: config.RateLimit,
		Ctime:     config.Ctime,
		Utime:     config.Utime,
	}

	// 转换JSON字段
	if config.ChannelConfig != "" {
		daoConfig.ChannelConfig = []byte(config.ChannelConfig)
	}

	if config.TxnConfig != "" {
		daoConfig.TxnConfig = []byte(config.TxnConfig)
	}

	if config.Quota != "" {
		daoConfig.Quota = []byte(config.Quota)
	}

	if config.RetryPolicy != "" {
		daoConfig.RetryPolicy = []byte(config.RetryPolicy)
	}

	// 调用DAO层保存方法
	return b.configDao.SaveNonZeroConfig(ctx, daoConfig)
}

// GetByOwner 根据业务方信息获取配置
func (b *businessConfigRepository) GetByOwner(ctx context.Context, ownerID int64, ownerType string) (domain.BusinessConfig, error) {
	// 构建查询条件并查询数据
	// 这里简化实现，实际代码可能需要通过DAO层接口实现或直接查询
	var configs []domain.BusinessConfig
	var total int64

	// 使用List方法查询
	configs, total, err := b.List(ctx, 0, 1, map[string]interface{}{
		"owner_id":   ownerID,
		"owner_type": ownerType,
	})

	if err != nil {
		return domain.BusinessConfig{}, err
	}

	if total == 0 || len(configs) == 0 {
		return domain.BusinessConfig{}, egorm.ErrRecordNotFound
	}

	return configs[0], nil
}

// List 列出业务配置
func (b *businessConfigRepository) List(ctx context.Context, offset, limit int, filters map[string]interface{}) ([]domain.BusinessConfig, int64, error) {
	// 这里是模拟实现，实际应通过DAO层接口或直接查询数据库
	// 在此处，我们创建一个空数组和固定的总数
	// 实际实现应该查询数据库

	// 由于DAO层没有提供List方法，我们这里模拟返回空数据
	// 实际开发中应该扩展DAO层接口并实现真正的查询

	return []domain.BusinessConfig{}, 0, nil
}
