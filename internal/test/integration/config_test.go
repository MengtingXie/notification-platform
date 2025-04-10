package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/pkg/retry"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ego-component/egorm"
	ca "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config"
	configIoc "gitee.com/flycash/notification-platform/internal/test/integration/ioc/config"
	"gitee.com/flycash/notification-platform/internal/test/ioc"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BusinessConfigTestSuite struct {
	suite.Suite
	localCache *ca.Cache
	redisCache *redis.Client
	db         *egorm.Component
	svc        config.BusinessConfigService
}

func (s *BusinessConfigTestSuite) SetupSuite() {
	localCache := ca.New(10*time.Minute, 10*time.Minute)
	s.svc = configIoc.InitConfigService(localCache)
	s.localCache = localCache
	s.redisCache = ioc.InitRedisClient()
	s.db = ioc.InitDB()
	err := dao.InitTables(s.db)
	require.NoError(s.T(), err)
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
			RetryPolicy: &retry.Config{
				Type: "fixed",
				FixedInterval: &retry.FixedIntervalConfig{
					Interval:   10,
					MaxRetries: 3,
				},
			},
		},
		RateLimit: 2000,
		Quota: &domain.QuotaConfig{
			Monthly: domain.MonthlyConfig{
				SMS:   100,
				EMAIL: 100,
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
				RetryPolicy: &retry.Config{
					Type: "fixed",
					FixedInterval: &retry.FixedIntervalConfig{
						Interval:   10,
						MaxRetries: 3,
					},
				},
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
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
				RetryPolicy: &retry.Config{
					Type: "fixed",
					FixedInterval: &retry.FixedIntervalConfig{
						Interval:   10,
						MaxRetries: 3,
					},
				},
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
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
				RetryPolicy: &retry.Config{
					Type: "fixed",
					FixedInterval: &retry.FixedIntervalConfig{
						Interval:   10,
						MaxRetries: 3,
					},
				},
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
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
				RetryPolicy: &retry.Config{
					Type: "fixed",
					FixedInterval: &retry.FixedIntervalConfig{
						Interval:   10,
						MaxRetries: 3,
					},
				},
			},
			RateLimit: 2000,
			Quota: &domain.QuotaConfig{
				Monthly: domain.MonthlyConfig{
					SMS:   100,
					EMAIL: 100,
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
	key := fmt.Sprintf("config:%d", testConfig.ID)
	err = s.redisCache.Del(ctx, key).Err()
	require.NoError(t, err)
	s.localCache.Delete(key)
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
				s.checkBusinessConfig(t, s.createTestConfig())
				// 清理：删除创建的配置
				err := s.svc.Delete(ctx, 5)
				assert.NoError(t, err, "删除业务配置应成功")
				err = s.redisCache.Del(context.Background(), "config:5").Err()
				assert.NoError(t, err, "删除业务配置应成功")
				s.localCache.Delete("config:5")
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
				OwnerID:   1003,
				OwnerType: "person",
				ChannelConfig: &domain.ChannelConfig{
					Channels: []domain.ChannelItem{
						{
							Channel:  "SMS",
							Priority: 2,
							Enabled:  true,
						},
						{
							Channel:  "EMAIL",
							Priority: 3,
							Enabled:  true,
						},
					},
				},
				TxnConfig: &domain.TxnConfig{
					ServiceName:  "newServiceName",
					InitialDelay: 20,
					RetryPolicy: &retry.Config{
						Type: "fixed",
						FixedInterval: &retry.FixedIntervalConfig{
							Interval:   40,
							MaxRetries: 4,
						},
					},
				},
				RateLimit: 6000,
				Quota: &domain.QuotaConfig{
					Monthly: domain.MonthlyConfig{
						SMS:   200,
						EMAIL: 200,
					},
				},
			},
			after: func(t *testing.T) {
				s.checkBusinessConfig(t, domain.BusinessConfig{
					ID:        5,
					OwnerID:   1003,
					OwnerType: "person",
					ChannelConfig: &domain.ChannelConfig{
						Channels: []domain.ChannelItem{
							{
								Channel:  "SMS",
								Priority: 2,
								Enabled:  true,
							},
							{
								Channel:  "EMAIL",
								Priority: 3,
								Enabled:  true,
							},
						},
					},
					TxnConfig: &domain.TxnConfig{
						ServiceName:  "newServiceName",
						InitialDelay: 20,
						RetryPolicy: &retry.Config{
							Type: "fixed",
							FixedInterval: &retry.FixedIntervalConfig{
								Interval:   40,
								MaxRetries: 4,
							},
						},
					},
					RateLimit: 6000,
					Quota: &domain.QuotaConfig{
						Monthly: domain.MonthlyConfig{
							SMS:   200,
							EMAIL: 200,
						},
					},
				})

				// 清理：删除创建的配置
				err := s.svc.Delete(ctx, 5)
				assert.NoError(t, err, "删除业务配置应成功")
				err = s.redisCache.Del(context.Background(), "config:5").Err()
				assert.NoError(t, err, "删除业务配置应成功")
				s.localCache.Delete("config:5")
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
			time.Sleep(100 * time.Millisecond)
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
			conf, err := s.svc.GetByID(ctx, id)
			require.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assertBusinessConfig(t, tc.wantConfig, conf)
			s.checkBusinessConfig(t, conf)
			err = s.svc.Delete(ctx, 5)
			assert.NoError(t, err, "删除业务配置应成功")
			err = s.redisCache.Del(context.Background(), "config:5").Err()
			assert.NoError(t, err, "删除业务配置应成功")
			s.localCache.Delete("config:5")
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

				_, ok := s.localCache.Get("config:5")
				require.False(t, ok)
				res := s.redisCache.Get(context.Background(), "config:5")
				assert.Equal(t, redis.Nil, res.Err())
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
			time.Sleep(100 * time.Millisecond)
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
	wantConfig.Ctime = 0
	actualConfig.Utime = 0
	wantConfig.Utime = 0
	assert.Equal(t, wantConfig, actualConfig)
}

func (s *BusinessConfigTestSuite) checkBusinessConfig(t *testing.T, wantConfig domain.BusinessConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var cfgDao dao.BusinessConfig
	err := s.db.WithContext(ctx).Where("id = ?", wantConfig.ID).First(&cfgDao).Error
	assert.NoError(t, err)
	conf := s.toDomain(cfgDao)

	key := fmt.Sprintf("config:%d", wantConfig.ID)
	v, ok := s.localCache.Get(key)
	require.True(t, ok)
	assertBusinessConfig(t, conf, v.(domain.BusinessConfig))

	res := s.redisCache.Get(context.Background(), key)
	require.NoError(t, res.Err())
	var redisConf domain.BusinessConfig
	configStr := res.Val()
	err = json.Unmarshal([]byte(configStr), &redisConf)
	require.NoError(t, res.Err())
	assertBusinessConfig(t, conf, redisConf)

}

func (s *BusinessConfigTestSuite) toDomain(daoConfig dao.BusinessConfig) domain.BusinessConfig {
	domainCfg := domain.BusinessConfig{
		ID:        daoConfig.ID,
		OwnerID:   daoConfig.OwnerID,
		OwnerType: daoConfig.OwnerType,
		RateLimit: daoConfig.RateLimit,
		Ctime:     daoConfig.Ctime,
		Utime:     daoConfig.Utime,
	}
	if daoConfig.ChannelConfig.Valid {
		domainCfg.ChannelConfig = unmarsal[domain.ChannelConfig](daoConfig.ChannelConfig.String)
	}
	if daoConfig.TxnConfig.Valid {
		domainCfg.TxnConfig = unmarsal[domain.TxnConfig](daoConfig.TxnConfig.String)
	}
	if daoConfig.Quota.Valid {
		domainCfg.Quota = unmarsal[domain.QuotaConfig](daoConfig.Quota.String)
	}
	return domainCfg
}

func unmarsal[T any](v string) *T {
	var t T
	_ = json.Unmarshal([]byte(v), &t)
	return &t
}
