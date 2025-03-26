//go:build e2e

package integration

import (
	"context"
	"testing"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/integration/startup"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
	"github.com/ego-component/egorm"

	"github.com/stretchr/testify/require"

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

// TestServiceCreate 测试创建业务配置
func (s *BusinessConfigTestSuite) TestServiceSaveConfig() {
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

	t.Run("id为0", func(t *testing.T) {
		invalidConfig := domain.BusinessConfig{
			ID: 0, // 无效ID
		}
		err := s.svc.SaveConfig(ctx, invalidConfig)
		assert.Equal(t, service.ErrIDNotSet, err)
	})
}

// TestBusinessConfigRead 测试读取业务配置
func (s *BusinessConfigTestSuite) TestServiceGetByID() {
	testcases := []struct {
		name       string
		before     func(t *testing.T) int64
		wantConfig domain.BusinessConfig
		wantErr    error
	}{
		{
			name: "成功获取",
			before: func(t *testing.T) int64 {
				return s.createConfigAndGetID(t)
			},
			wantConfig: s.createTestConfig(),
		},
		{
			name: "未存在的id",
			before: func(t *testing.T) int64 {
				return 9999
			},
			wantErr: service.ErrConfigNotFound,
		},
	}
	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			id := tc.before(t)
			config, err := s.svc.GetByID(ctx, id)
			require.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assertBusinessConfig(t, tc.wantConfig, config)
			err = s.svc.Delete(ctx, 5)
			assert.NoError(t, err, "删除业务配置应成功")
		})
	}
}

func (s *BusinessConfigTestSuite) TestServiceGetByIDs() {
	t := s.T()
	ctx := context.Background()
	configList := s.createTestConfigList()
	for _, nconfig := range configList {
		err := s.svc.SaveConfig(ctx, nconfig)
		require.NoError(t, err)
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
	for i := 1; i <= 4; i++ {
		err = s.svc.Delete(ctx, int64(i))
		require.NoError(t, err)
	}
}

// TestBusinessConfigDelete 测试删除业务配置
func (s *BusinessConfigTestSuite) TestServiceDelete() {
	testcases := []struct {
		name    string
		id      int64
		before  func(t *testing.T)
		after   func(t *testing.T)
		wantErr error
	}{
		{
			name: "正常删除",
			id:   5,
			before: func(t *testing.T) {
				s.createConfigAndGetID(t)
			},
			after: func(t *testing.T) {
				_, err := s.svc.GetByID(context.Background(), 5)
				assert.Equal(t, service.ErrConfigNotFound, err, "应返回配置不存在错误")
			},
		},
		{
			name: "删除id不存在的数据",
			id:   99999,
			before: func(t *testing.T) {
			},
			after:   func(t *testing.T) {},
			wantErr: egorm.ErrRecordNotFound,
		},
	}
	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			tc.before(t)
			err := s.svc.Delete(ctx, tc.id)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			tc.after(t)
		})
	}
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
