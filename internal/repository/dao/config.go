package dao

import (
	"context"
	"database/sql"
	"time"

	"github.com/ego-component/egorm"
	"gorm.io/gorm/clause"
)

// BusinessConfig 业务配置表
type BusinessConfig struct {
	ID             int64          `gorm:"primaryKey;type:BIGINT;comment:'业务标识'"`
	OwnerID        int64          `gorm:"type:BIGINT;comment:'业务方'"`
	OwnerType      string         `gorm:"type:ENUM('person', 'organization');comment:'业务方类型：person-个人,organization-组织'"`
	ChannelConfig  sql.NullString `gorm:"type:JSON;comment:'{\"channels\":[{\"channel\":\"SMS\", \"priority\":\"1\",\"enabled\":\"true\"},{\"channel\":\"EMAIL\", \"priority\":\"2\",\"enabled\":\"true\"}]}'"`
	TxnConfig      sql.NullString `gorm:"type:JSON;comment:'事务配置'"`
	RateLimit      int            `gorm:"type:INT;DEFAULT:1000;comment:'每秒最大请求数'"`
	Quota          sql.NullString `gorm:"type:JSON;comment:'{\"monthly\":{\"SMS\":100000,\"EMAIL\":500000}}'"`
	CallbackConfig sql.NullString `gorm:"type:JSON;comment:'回调配置，通知平台回调业务方通知异步请求结果'"`
	Ctime          int64
	Utime          int64
}

// TableName 重命名表
func (BusinessConfig) TableName() string {
	return "business_configs"
}

type BusinessConfigDAO interface {
	GetByIDs(ctx context.Context, id []int64) (map[int64]BusinessConfig, error)
	GetByID(ctx context.Context, id int64) (BusinessConfig, error)
	Delete(ctx context.Context, id int64) error
	SaveConfig(ctx context.Context, config BusinessConfig) (BusinessConfig, error)
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
	err := b.db.WithContext(ctx).Where("id in (?)", ids).Find(&configs).Error
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
	return nil
}

// SaveConfig 保存业务配置
func (b *businessConfigDAO) SaveConfig(ctx context.Context, config BusinessConfig) (BusinessConfig, error) {
	now := time.Now().UnixMilli()
	config.Ctime = now
	config.Utime = now
	// 使用upsert语句，如果记录存在则更新，不存在则插入
	db := b.db.WithContext(ctx)
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},           // 根据ID判断冲突
		DoUpdates: clause.AssignmentColumns(updateColumns), // 只更新指定的非空列
	}).Create(&config)
	if result.Error != nil {
		return BusinessConfig{}, result.Error
	}
	return config, nil
}

var updateColumns = []string{
	"owner_id",
	"owner_type",
	"channel_config",
	"quota",
	"txn_config",
	"rate_limit",
	"retry_policy",
	"utime",
}
