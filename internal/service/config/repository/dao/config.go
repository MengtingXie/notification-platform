package dao

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/pkg/dao"
	"github.com/ego-component/egorm"
)

// BusinessConfig 业务配置表
type BusinessConfig struct {
	ID            int64    `gorm:"primaryKey;type:BIGINT;comment:'业务标识'"`
	OwnerID       int64    `gorm:"type:BIGINT;comment:'业务方'"`
	OwnerType     string   `gorm:"type:ENUM('person', 'organization');comment:'业务方类型：person-个人,organization-组织'"`
	ChannelConfig dao.JSON `gorm:"type:JSON;NOT NULL;comment:'{\"allowed_channels\":[\"SMS\",\"EMAIL\"], \"default\":\"SMS\"}'"`
	TxnConfig     dao.JSON `gorm:"type:JSON;NOT NULL;default:'{}';comment:'事务配置'"`
	RateLimit     int      `gorm:"type:INT;DEFAULT:1000;comment:'每秒最大请求数'"`
	Quota         dao.JSON `gorm:"type:JSON;comment:'{\"monthly\":{\"SMS\":100000,\"EMAIL\":500000}}'"`
	RetryPolicy   dao.JSON `gorm:"type:JSON;comment:'{\"max_attempts\":3, \"backoff\":\"EXPONENTIAL\"}'"`
	Ctime         int64
	Utime         int64
}

// TableName 重命名表
func (BusinessConfig) TableName() string {
	return "business_config"
}

type BusinessConfigDAO interface {
	GetByIDs(ctx context.Context, id []int64) (map[int64]BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	// SaveNonZeroConfig 保存非零字段
	SaveNonZeroConfig(ctx context.Context, config BusinessConfig) error
}

// Implementation of the BusinessConfigDAO interface
type businessConfigDAO struct {
	db *egorm.Component
}

// NewBusinessConfigDAO 创建一个新的BusinessConfigDAO实例
func NewBusinessConfigDAO(db *egorm.Component) BusinessConfigDAO {
	return &businessConfigDAO{
		db: db,
	}
}
func (b *businessConfigDAO) GetByID(ctx context.Context, id int64) (BusinessConfig, error) {
	var config BusinessConfig

	// 根据ID查询业务配置
	err := b.db.WithContext(ctx).Where("id = ?", id).First(&config).Error
	if err != nil {
		return BusinessConfig{}, err
	}

	return config, nil
}

// GetByIDs 根据ID获取业务配置信息
func (b *businessConfigDAO) GetByIDs(ctx context.Context, ids []int64) (map[int64]BusinessConfig, error) {
	var configs []BusinessConfig
	// 根据ID查询业务配置
	err := b.db.WithContext(ctx).Where("id in (?)", ids).First(&configs).Error
	if err != nil {
		return nil, err
	}
	configMap := make(map[int64]BusinessConfig, len(ids))
	for _, config := range configs {
		configMap[config.ID] = config
	}
	return configMap, nil
}

// Delete 根据ID删除业务配置
func (b *businessConfigDAO) Delete(ctx context.Context, id int64) error {
	// 执行删除操作
	result := b.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&BusinessConfig{})
	if result.Error != nil {
		return result.Error
	}

	// 检查是否有记录被删除
	if result.RowsAffected == 0 {
		return egorm.ErrRecordNotFound
	}

	return nil
}

// SaveNonZeroConfig 保存业务配置（新增或更新非零字段）
func (b *businessConfigDAO) SaveNonZeroConfig(ctx context.Context, config BusinessConfig) error {
	// 判断是新增还是更新操作
	if config.ID == 0 {
		// 新增记录
		return b.db.WithContext(ctx).Create(&config).Error
	} else {
		// 更新记录 - 使用Updates方法会自动过滤零值字段
		result := b.db.WithContext(ctx).Model(&BusinessConfig{}).Where("id = ?", config.ID).Updates(config)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return egorm.ErrRecordNotFound
		}

		return nil
	}
}
