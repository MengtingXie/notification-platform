package integration

import (
	"context"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/test/ioc"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BusinessConfigTestSuite struct {
	suite.Suite
	svc config.BusinessConfigService
}

func (s *BusinessConfigTestSuite) SetupSuite() {
	db := ioc.InitDB()
	err := dao.InitTables(db)
	require.NoError(s.T(), err)
	configDao := dao.NewBusinessConfigDAO(db)
	repo := repository.NewBusinessConfigRepository(configDao)
	svc := config.NewBusinessConfigService(repo)
	s.svc = svc
}

func (s *BusinessConfigTestSuite) createTestConfig() domain.BusinessConfig {
	return domain.BusinessConfig{
		ID:        5,
		OwnerID:   1001,
		OwnerType: "person",
		ChannelConfig: &domain.ChannelConfig{
			Channels: []domain.ChannelItem{
				{
					Channel:  "SMS",
					Priority: 1,
					Enabled:  true,
				},
				{
					Channel:  "EMAIL",
					Priority: 2,
					Enabled:  true,
				},
			},
		},
		TxnConfig: &domain.TxnConfig{
			ServiceName:  "serviceName",
			InitialDelay: 10,
		},
		RateLimit: 2000,
		Quota: &domain.QuotaConfig{
			Monthly: domain.MonthlyConfig{
				SMS:   100,
				EMAIL: 100,
			},
		},
		RetryPolicy: &domain.RetryConfig{
			Type: "fixed",
			FixedInterval: &domain.FixedIntervalConfig{
				Interval:   10,
				MaxRetries: 3,
			},
		},
	}
}

func (s *BusinessConfigTestSuite) createTestConfigList() []domain.BusinessConfig {
	return []domain.BusinessConfig{
		{
			ID:        1,
			OwnerID:   1001,
			OwnerType: "person",
			ChannelConfig: &domain.ChannelConfig{
				Channels: []domain.ChannelItem{
					{
						Channel:  "SMS",
						Priority: 1,
						Enabled:  true,
					},
					{
						Channel:  "EMAIL",
						Priority: 2,
						Enabled:  true,
					},
				},
			},
			TxnConfig: &domain.TxnConfig{
				ServiceName:  "serviceName",
				InitialDelay: 10,
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
				},
			},

			RetryPolicy: &domain.RetryConfig{
				Type: "fixed",
				FixedInterval: &domain.FixedIntervalConfig{
					Interval:   10,
					MaxRetries: 3,
				},
			},
		},
		{
			ID:        2,
			OwnerID:   1002,
			OwnerType: "person",
			ChannelConfig: &domain.ChannelConfig{
				Channels: []domain.ChannelItem{
					{
						Channel:  "SMS",
						Priority: 1,
						Enabled:  true,
					},
					{
						Channel:  "EMAIL",
						Priority: 2,
						Enabled:  true,
					},
				},
			},
			TxnConfig: &domain.TxnConfig{
				ServiceName:  "serviceName",
				InitialDelay: 10,
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
				},
			},

			RetryPolicy: &domain.RetryConfig{
				Type: "fixed",
				FixedInterval: &domain.FixedIntervalConfig{
					Interval:   10,
					MaxRetries: 3,
				},
			},
		},
		{
			ID:        3,
			OwnerID:   1003,
			OwnerType: "person",
			ChannelConfig: &domain.ChannelConfig{
				Channels: []domain.ChannelItem{
					{
						Channel:  "SMS",
						Priority: 1,
						Enabled:  true,
					},
					{
						Channel:  "EMAIL",
						Priority: 2,
						Enabled:  true,
					},
				},
			},
			TxnConfig: &domain.TxnConfig{
				ServiceName:  "serviceName",
				InitialDelay: 10,
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
				},
			},

			RetryPolicy: &domain.RetryConfig{
				Type: "fixed",
				FixedInterval: &domain.FixedIntervalConfig{
					Interval:   10,
					MaxRetries: 3,
				},
			},
		},
		{
			ID:        4,
			OwnerID:   1004,
			OwnerType: "person",
			ChannelConfig: &domain.ChannelConfig{
				Channels: []domain.ChannelItem{
					{
						Channel:  "SMS",
						Priority: 1,
						Enabled:  true,
					},
					{
						Channel:  "EMAIL",
						Priority: 2,
						Enabled:  true,
					},
				},
			},
			TxnConfig: &domain.TxnConfig{
				ServiceName:  "serviceName",
				InitialDelay: 10,
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
				},
			},

			RetryPolicy: &domain.RetryConfig{
				Type: "fixed",
				FixedInterval: &domain.FixedIntervalConfig{
					Interval:   10,
					MaxRetries: 3,
				},
			},
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
	testcases := []struct {
		name    string
		before  func(t *testing.T)
		req     domain.BusinessConfig
		after   func(t *testing.T)
		wantErr error
	}{
		{
			name:   "新增",
			before: func(t *testing.T) {},
			req:    s.createTestConfig(),
			after: func(t *testing.T) {
				// 验证创建结果
				config, err := s.svc.GetByID(ctx, 5)
				assert.NoError(t, err, "查询单个业务配置应成功")
				assertBusinessConfig(t, s.createTestConfig(), config)

				// 清理：删除创建的配置
				err = s.svc.Delete(ctx, 5)
				assert.NoError(t, err, "删除业务配置应成功")
			},
		},
		{
			name: "更新",
			before: func(t *testing.T) {
				testConfig := s.createTestConfig()
				err := s.svc.SaveConfig(ctx, testConfig)
				assert.NoError(t, err, "创建业务配置应成功")
			},
			req: domain.BusinessConfig{
				ID:        5,
				OwnerID:   1002,
				OwnerType: "person",
				ChannelConfig: &domain.ChannelConfig{
					Channels: []domain.ChannelItem{
						{
							Channel:  "SMS",
							Priority: 1,
							Enabled:  true,
						},
						{
							Channel:  "EMAIL",
							Priority: 2,
							Enabled:  true,
						},
					},
				},
				TxnConfig: &domain.TxnConfig{
					ServiceName:  "serviceName",
					InitialDelay: 10,
				},
				RateLimit: 3000,
				Quota: &domain.QuotaConfig{
					Monthly: domain.MonthlyConfig{
						SMS:   100,
						EMAIL: 100,
					},
				},
				RetryPolicy: &domain.RetryConfig{
					Type: "fixed",
					FixedInterval: &domain.FixedIntervalConfig{
						Interval:   10,
						MaxRetries: 3,
					},
				},
			},
			after: func(t *testing.T) {
				config, err := s.svc.GetByID(ctx, 5)
				assert.NoError(t, err)
				assertBusinessConfig(t, domain.BusinessConfig{
					ID:        5,
					OwnerID:   1002,
					OwnerType: "person",
					ChannelConfig: &domain.ChannelConfig{
						Channels: []domain.ChannelItem{
							{
								Channel:  "SMS",
								Priority: 1,
								Enabled:  true,
							},
							{
								Channel:  "EMAIL",
								Priority: 2,
								Enabled:  true,
							},
						},
					},
					TxnConfig: &domain.TxnConfig{
						ServiceName:  "serviceName",
						InitialDelay: 10,
					},
					RateLimit: 3000,
					Quota: &domain.QuotaConfig{
						Monthly: domain.MonthlyConfig{
							SMS:   100,
							EMAIL: 100,
						},
					},
					RetryPolicy: &domain.RetryConfig{
						Type: "fixed",
						FixedInterval: &domain.FixedIntervalConfig{
							Interval:   10,
							MaxRetries: 3,
						},
					},
				}, config)
				// 清理：删除创建的配置
				err = s.svc.Delete(ctx, 5)
				assert.NoError(t, err, "删除业务配置应成功")
			},
		},
		{
			name:   "id为0",
			before: func(t *testing.T) {},
			req: domain.BusinessConfig{
				ID: 0,
			},
			after:   func(t *testing.T) {},
			wantErr: config.ErrIDNotSet,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.before(t)
			err := s.svc.SaveConfig(ctx, tc.req)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			tc.after(t)
		})
	}
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
			wantErr: config.ErrConfigNotFound,
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
				assert.Equal(t, config.ErrConfigNotFound, err, "应返回配置不存在错误")
			},
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
