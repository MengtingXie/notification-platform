package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"gitee.com/flycash/notification-platform/internal/service/config/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/integration/startup"
	"gitee.com/flycash/notification-platform/internal/service/config/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BusinessConfigTestSuite struct {
	suite.Suite
	svc service.BusinessConfigService
}

func (s *BusinessConfigTestSuite) SetupSuite() {
	module := startup.InitService()
	s.svc = module.Svc
}

func (s *BusinessConfigTestSuite) createTestConfig() domain.BusinessConfig {
	return domain.BusinessConfig{
		ID:            5,
		OwnerID:       1001,
		OwnerType:     "person",
		ChannelConfig: `{"channelconfig": "key"}`,
		TxnConfig:     `{"txnconfig": "key"}`,
		RateLimit:     2000,
		Quota:         `{"quota": "key"}`,
		RetryPolicy:   `{"retryconfig": "key"}`,
	}
}

func (s *BusinessConfigTestSuite) createTestConfigList() []domain.BusinessConfig {
	return []domain.BusinessConfig{
		{
			ID:            1,
			OwnerID:       1001,
			OwnerType:     "person",
			ChannelConfig: `{"channelconfig1": "key1"}`,
			TxnConfig:     `{"txnconfig1": "key1"}`,
			RateLimit:     2001,
			Quota:         `{"quota1": "key1"}`,
			RetryPolicy:   `{"retryconfig1": "key1"}`,
		},
		{
			ID:            2,
			OwnerID:       1002,
			OwnerType:     "person",
			ChannelConfig: `{"channelconfig2": "key2"}`,
			TxnConfig:     `{"txnconfig2": "key2"}`,
			RateLimit:     2001,
			Quota:         `{"quota2": "key2"}`,
			RetryPolicy:   `{"retryconfig2": "key2"}`,
		},
		{
			ID:            3,
			OwnerID:       1003,
			OwnerType:     "person",
			ChannelConfig: `{"channelconfig3": "key3"}`,
			TxnConfig:     `{"txnconfig3": "key3"}`,
			RateLimit:     2003,
			Quota:         `{"quota3": "key3"}`,
			RetryPolicy:   `{"retryconfig3": "key3"}`,
		},
		{
			ID:            4,
			OwnerID:       1004,
			OwnerType:     "person",
			ChannelConfig: `{"channelconfig4": "key4"}`,
			TxnConfig:     `{"txnconfig4": "key4"}`,
			RateLimit:     2003,
			Quota:         `{"quota4": "key4"}`,
			RetryPolicy:   `{"retryconfig4": "key4"}`,
		},
	}
}

// 创建配置并返回ID
func (s *BusinessConfigTestSuite) createConfigAndGetID(t *testing.T) int64 {
	testConfig := s.createTestConfig()
	ctx := context.Background()
	// 创建配置
	err := s.svc.SaveConfig(ctx, testConfig)
	assert.NoError(t, err, "创建业务配置应成功")
	return 5
}

// TestBusinessConfigCreate 测试创建业务配置
func (s *BusinessConfigTestSuite) TestBusinessConfigCreate() {
	t := s.T()
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		// 创建测试数据
		testConfig := s.createTestConfig()

		// 测试SaveNonZeroConfig - 创建
		err := s.svc.SaveConfig(ctx, testConfig)
		assert.NoError(t, err, "创建业务配置应成功")
		// 验证创建结果
		config, err := s.svc.GetByID(ctx, 5)
		assert.NoError(t, err, "查询单个业务配置应成功")
		assert.Equal(t, testConfig.OwnerID, config.OwnerID)
		assert.Equal(t, testConfig.OwnerType, config.OwnerType)
		assert.Equal(t, testConfig.ChannelConfig, config.ChannelConfig)
		assert.Equal(t, testConfig.RateLimit, config.RateLimit)

		// 清理：删除创建的配置
		err = s.svc.Delete(ctx, 5)
		assert.NoError(t, err, "删除业务配置应成功")
	})

	t.Run("Update", func(t *testing.T) {
		// 先创建配置
		testConfig := s.createTestConfig()
		err := s.svc.SaveConfig(ctx, testConfig)
		assert.NoError(t, err, "创建业务配置应成功")

		newconfig := domain.BusinessConfig{
			ID:            5,
			OwnerID:       1002,
			OwnerType:     "person",
			ChannelConfig: `{"newchannelconfig": "key"}`,
			TxnConfig:     `{"newtxnconfig": "key"}`,
			RateLimit:     3000,
			Quota:         `{"newquota": "key"}`,
			RetryPolicy:   `{"newretryconfig": "key"}`,
		}

		err = s.svc.SaveConfig(ctx, newconfig)
		assert.NoError(t, err, "更新业务配置应成功")

		// 验证更新结果
		updatedConfig, err := s.svc.GetByID(ctx, 5)
		require.True(t, updatedConfig.Utime > 0)
		require.True(t, updatedConfig.Ctime > 0)
		updatedConfig.Ctime = 0
		updatedConfig.Utime = 0
		assert.Equal(t, newconfig, updatedConfig)
		// 清理：删除创建的配置
		err = s.svc.Delete(ctx, 5)
		assert.NoError(t, err, "删除业务配置应成功")
	})
}

// TestBusinessConfigRead 测试读取业务配置
func (s *BusinessConfigTestSuite) TestBusinessConfigRead() {
	t := s.T()
	ctx := context.Background()
	// 先创建配置
	newID := s.createConfigAndGetID(t)
	testConfig := s.createTestConfig() // 用于验证

	// 测试GetByID
	config, err := s.svc.GetByID(ctx, newID)
	assert.NoError(t, err, "查询单个业务配置应成功")
	assertBusinessConfig(t, testConfig, config)

	configList := s.createTestConfigList()
	for _, nconfig := range configList {
		err = s.svc.SaveConfig(ctx, nconfig)
	}

	// 测试GetByIDs
	configMap, err := s.svc.GetByIDs(ctx, []int64{1, 2, 3, 4})
	for _, wantconfig := range configList {
		v, ok := configMap[wantconfig.ID]
		require.True(t, ok)
		assertBusinessConfig(t, wantconfig, v)
	}

	// 测试查询不存在的ID
	nonExistConfigs, err := s.svc.GetByIDs(ctx, []int64{9999999})
	assert.NoError(t, err, "查询不存在的ID应返回空map，不应报错")
	assert.Empty(t, nonExistConfigs[9999999], "不存在的ID对应值应为空")

	// 清理：删除创建的配置
	for i := 1; i <= 5; i++ {
		err = s.svc.Delete(ctx, int64(i))
		require.NoError(t, err)
	}
}

// TestBusinessConfigDelete 测试删除业务配置
func (s *BusinessConfigTestSuite) TestBusinessConfigDelete() {
	t := s.T()
	ctx := context.Background()
	// 先创建配置
	newID := s.createConfigAndGetID(t)

	// 测试Delete
	err := s.svc.Delete(ctx, newID)
	assert.NoError(t, err, "删除业务配置应成功")

	// 验证删除结果
	_, err = s.svc.GetByID(ctx, newID)
	assert.Error(t, err, "查询已删除的配置应返回错误")
	assert.Equal(t, service.ErrConfigNotFound, err, "应返回配置不存在错误")
}

func (s *BusinessConfigTestSuite) TestInvalidParameters() {
	t := s.T()
	ctx := context.Background()
	// 测试GetByID无效参数
	_, err := s.svc.GetByID(ctx, 0)
	assert.Error(t, err)
	assert.Equal(t, service.ErrInvalidParameter, err)

	// 测试Delete无效参数
	err = s.svc.Delete(ctx, 0)
	assert.Error(t, err)
	assert.Equal(t, service.ErrInvalidParameter, err)

	// 测试SaveNonZeroConfig无效参数
	invalidConfig := domain.BusinessConfig{
		ID: 0, // 无效ID
	}
	err = s.svc.SaveConfig(ctx, invalidConfig)
	assert.Error(t, err)
	assert.Equal(t, service.ErrIDNotSet, err)

	// 新增时缺少必填参数
	emptyConfig := domain.BusinessConfig{}
	err = s.svc.SaveConfig(ctx, emptyConfig)
	assert.Error(t, err)
	assert.Equal(t, service.ErrIDNotSet, err)
}

func TestBusinessConfigService(t *testing.T) {
	suite.Run(t, new(BusinessConfigTestSuite))
}

func assertBusinessConfig(t *testing.T, wantConfig domain.BusinessConfig, actualConfig domain.BusinessConfig) {
	require.True(t, actualConfig.Ctime > 0)
	require.True(t, actualConfig.Utime > 0)
	actualConfig.Ctime = 0
	actualConfig.Utime = 0
	assert.Equal(t, wantConfig, actualConfig)
}
