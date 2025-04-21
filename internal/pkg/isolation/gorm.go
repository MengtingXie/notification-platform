package isolation

// 使用装饰器，封装不同的gorm.db,要求核心业务使用的gorm.db 使用的连接数多一点，非核心业务使用的gorm.db的连接数少一点。

import (
	"context"
	"sync"
	"time"

	"github.com/ego-component/egorm"
	"gorm.io/gorm"
)

// BusinessType 表示业务类型
type BusinessType string

const (
	// CoreBusiness 核心业务类型
	CoreBusiness BusinessType = "core"
	// NonCoreBusiness 非核心业务类型
	NonCoreBusiness BusinessType = "non-core"
)

// ConnectionPoolConfig 数据库连接池配置
type ConnectionPoolConfig struct {
	// 最大连接数
	MaxOpenConns int
	// 最大空闲连接数
	MaxIdleConns int
	// 连接最大存活时间
	ConnMaxLifetime time.Duration
	// 连接最大空闲时间
	ConnMaxIdleTime time.Duration
}

const (
	CoreMaxOpenConns       = 100
	CoreMaxIdleConns       = 20
	CoreConnMaxLifetime    = 30 * time.Minute
	NonCoreMaxOpenConns    = 20
	NonCoreMaxIdleConns    = 4
	NonCoreConnMaxLifetime = 15 * time.Minute
)

// DefaultCoreConfig 核心业务的默认连接池配置
func DefaultCoreConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConns:    CoreMaxOpenConns,
		MaxIdleConns:    CoreMaxIdleConns,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: CoreConnMaxLifetime,
	}
}

// DefaultNonCoreConfig 非核心业务的默认连接池配置
func DefaultNonCoreConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConns:    NonCoreMaxOpenConns,
		MaxIdleConns:    NonCoreMaxIdleConns,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: NonCoreConnMaxLifetime,
	}
}

// GORMDecorator GORM装饰器
type GORMDecorator struct {
	// 核心业务DB实例
	coreDB *egorm.Component
	// 非核心业务DB实例
	nonCoreDB *egorm.Component
	// 默认的数据源名称
	dataSourceName string
	// 互斥锁，防止并发问题
	mu sync.RWMutex
}

// NewGORMDecorator 创建新的GORM装饰器
// 参数:
// - dataSourceName: 配置中的数据源名称，如"mysql"
// - coreConfig: 核心业务的连接池配置
// - nonCoreConfig: 非核心业务的连接池配置
func NewGORMDecorator(dataSourceName string, coreConfig, nonCoreConfig ConnectionPoolConfig) (*GORMDecorator, error) {
	// 创建装饰器
	decorator := &GORMDecorator{
		dataSourceName: dataSourceName,
	}

	// 初始化核心业务DB实例
	coreDB := egorm.Load(dataSourceName).Build()
	if coreDB == nil {
		return nil, gorm.ErrInvalidDB
	}

	// 初始化非核心业务DB实例
	nonCoreDB := egorm.Load(dataSourceName).Build()
	if nonCoreDB == nil {
		return nil, gorm.ErrInvalidDB
	}

	// 配置核心业务的连接池
	if err := configureConnectionPool(coreDB, coreConfig); err != nil {
		return nil, err
	}

	// 配置非核心业务的连接池
	if err := configureConnectionPool(nonCoreDB, nonCoreConfig); err != nil {
		return nil, err
	}

	decorator.coreDB = coreDB
	decorator.nonCoreDB = nonCoreDB

	return decorator, nil
}

// configureConnectionPool 配置连接池参数
func configureConnectionPool(db *egorm.Component, config ConnectionPoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return nil
}

// RegisterPlugin 注册插件到所有DB实例
func (d *GORMDecorator) RegisterPlugin(plugin gorm.Plugin) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 注册插件到核心业务DB
	if err := d.coreDB.Use(plugin); err != nil {
		return err
	}

	// 注册插件到非核心业务DB
	if err := d.nonCoreDB.Use(plugin); err != nil {
		return err
	}

	return nil
}

// GetDB 获取指定业务类型的DB实例
func (d *GORMDecorator) GetDB(businessType BusinessType) *gorm.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()

	switch businessType {
	case CoreBusiness:
		return d.coreDB
	case NonCoreBusiness:
		return d.nonCoreDB
	default:
		// 默认返回核心业务DB
		return d.coreDB
	}
}

// GetCoreDB 获取核心业务的DB实例
func (d *GORMDecorator) GetCoreDB() *gorm.DB {
	return d.GetDB(CoreBusiness)
}

// GetNonCoreDB 获取非核心业务的DB实例
func (d *GORMDecorator) GetNonCoreDB() *gorm.DB {
	return d.GetDB(NonCoreBusiness)
}

// WithContext 返回带有上下文的核心业务DB实例
func (d *GORMDecorator) WithContext(ctx context.Context, businessType BusinessType) *gorm.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()

	switch businessType {
	case CoreBusiness:
		return d.coreDB.WithContext(ctx)
	case NonCoreBusiness:
		return d.nonCoreDB.WithContext(ctx)
	default:
		return d.coreDB.WithContext(ctx)
	}
}

// Close 关闭所有数据库连接
func (d *GORMDecorator) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var lastErr error

	// 关闭核心业务DB
	if d.coreDB != nil {
		sqlDB, err := d.coreDB.DB()
		if err != nil {
			lastErr = err
		} else {
			if err := sqlDB.Close(); err != nil {
				lastErr = err
			}
		}
	}

	// 关闭非核心业务DB
	if d.nonCoreDB != nil {
		sqlDB, err := d.nonCoreDB.DB()
		if err != nil {
			lastErr = err
		} else {
			if err := sqlDB.Close(); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}
