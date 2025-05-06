//go:build e2e

package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type QuotaCacheTestSuite struct {
	suite.Suite
	client *redis.Client
	cache  cache.QuotaCache
}

func (s *QuotaCacheTestSuite) SetupSuite() {
	s.client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	s.cache = NewQuotaCache(s.client)
}

func (s *QuotaCacheTestSuite) TearDownSuite() {
	s.client.FlushDB(s.T().Context())
	s.client.Close()
}

func (s *QuotaCacheTestSuite) SetupTest() {
	s.client.FlushDB(s.T().Context())
}

func (s *QuotaCacheTestSuite) TearDownTest() {
	s.client.FlushDB(s.T().Context())
}

func (s *QuotaCacheTestSuite) TestCreateOrUpdate() {
	// Use unique bizID for this test
	testQuota := domain.Quota{
		BizID:   1001,
		Channel: domain.ChannelSMS,
		Quota:   100,
	}

	// Test create
	err := s.cache.CreateOrUpdate(s.T().Context(), testQuota)
	s.NoError(err)

	storedQuota, err := s.cache.Find(s.T().Context(), testQuota.BizID, testQuota.Channel)
	s.NoError(err)
	s.Equal(testQuota, storedQuota)

	// Test update
	testQuota.Quota = 200
	err = s.cache.CreateOrUpdate(s.T().Context(), testQuota)
	s.NoError(err)

	storedQuota, err = s.cache.Find(s.T().Context(), testQuota.BizID, testQuota.Channel)
	s.NoError(err)
	s.Equal(testQuota, storedQuota)
}

func (s *QuotaCacheTestSuite) TestIncr() {
	// Use unique bizID for this test
	testQuota := domain.Quota{
		BizID:   2001,
		Channel: domain.ChannelSMS,
		Quota:   100,
	}
	err := s.cache.CreateOrUpdate(s.T().Context(), testQuota)
	s.NoError(err)

	// Test single increment
	err = s.cache.Incr(s.T().Context(), testQuota.BizID, testQuota.Channel, 50)
	s.NoError(err)

	storedQuota, err := s.cache.Find(s.T().Context(), testQuota.BizID, testQuota.Channel)
	s.NoError(err)
	s.Equal(int32(150), storedQuota.Quota)

	// Test increment from negative value
	testQuota2 := domain.Quota{
		BizID:   2002,
		Channel: domain.ChannelSMS,
		Quota:   -100, // Start with negative value
	}
	err = s.cache.CreateOrUpdate(s.T().Context(), testQuota2)
	s.NoError(err)

	// Increment by 150, final value should be 50
	err = s.cache.Incr(s.T().Context(), testQuota2.BizID, testQuota2.Channel, 150)
	s.NoError(err)

	storedQuota2, err := s.cache.Find(s.T().Context(), testQuota2.BizID, testQuota2.Channel)
	s.NoError(err)
	s.Equal(int32(150), storedQuota2.Quota)
}

func (s *QuotaCacheTestSuite) TestDecr() {
	// Use unique bizID for this test
	testQuota := domain.Quota{
		BizID:   3001,
		Channel: domain.ChannelSMS,
		Quota:   100,
	}
	err := s.cache.CreateOrUpdate(s.T().Context(), testQuota)
	s.NoError(err)

	// Test single decrement
	err = s.cache.Decr(s.T().Context(), testQuota.BizID, testQuota.Channel, 50)
	s.NoError(err)

	storedQuota, err := s.cache.Find(s.T().Context(), testQuota.BizID, testQuota.Channel)
	s.NoError(err)
	s.Equal(int32(50), storedQuota.Quota)

	// Test decrement below zero
	err = s.cache.Decr(s.T().Context(), testQuota.BizID, testQuota.Channel, 60)
	s.Error(err)
	s.ErrorIs(err, ErrQuotaLessThenZero)
}

func (s *QuotaCacheTestSuite) TestMutiIncr() {
	// Use unique bizIDs for this test
	quotas := []domain.Quota{
		{BizID: 5001, Channel: domain.ChannelSMS, Quota: 100},
		{BizID: 5002, Channel: domain.ChannelSMS, Quota: 200},
		{BizID: 5003, Channel: domain.ChannelSMS, Quota: -1000},
	}

	// Test batch create
	err := s.cache.CreateOrUpdate(s.T().Context(), quotas...)
	s.NoError(err)

	// Test batch increment
	items := []cache.IncrItem{
		{BizID: 5001, Channel: domain.ChannelSMS, Val: 50},
		{BizID: 5002, Channel: domain.ChannelSMS, Val: 100},
		{BizID: 5003, Channel: domain.ChannelSMS, Val: 150},
	}
	err = s.cache.MutiIncr(s.T().Context(), items)
	s.NoError(err)

	// Verify batch increment results
	expectedQuotas := []domain.Quota{
		{BizID: 5001, Channel: domain.ChannelSMS, Quota: 150},
		{BizID: 5002, Channel: domain.ChannelSMS, Quota: 300},
		{BizID: 5003, Channel: domain.ChannelSMS, Quota: 150},
	}

	for _, quota := range expectedQuotas {
		storedQuota, err := s.cache.Find(s.T().Context(), quota.BizID, quota.Channel)
		s.NoError(err)
		s.Equal(quota, storedQuota)
	}

	// Test batch increment with non-existent quota
	items = []cache.IncrItem{
		{BizID: 5004, Channel: domain.ChannelSMS, Val: 50},
	}
	err = s.cache.MutiIncr(s.T().Context(), items)
	s.NoError(err)

	// Verify non-existent quota is created
	storedQuota, err := s.cache.Find(s.T().Context(), 5004, domain.ChannelSMS)
	s.NoError(err)
	s.Equal(int32(50), storedQuota.Quota)
}

func (s *QuotaCacheTestSuite) TestMutiDecr() {
	// Use unique bizIDs for this test
	quotas := []domain.Quota{
		{BizID: 6001, Channel: domain.ChannelSMS, Quota: 100},
		{BizID: 6002, Channel: domain.ChannelSMS, Quota: 200},
		{BizID: 6003, Channel: domain.ChannelSMS, Quota: 300},
	}

	// Test batch create
	err := s.cache.CreateOrUpdate(s.T().Context(), quotas...)
	s.NoError(err)

	// Test batch decrement
	items := []cache.IncrItem{
		{BizID: 6001, Channel: domain.ChannelSMS, Val: 50},
		{BizID: 6002, Channel: domain.ChannelSMS, Val: 100},
		{BizID: 6003, Channel: domain.ChannelSMS, Val: 1500},
	}
	err = s.cache.MutiDecr(s.T().Context(), items)
	assert.ErrorAs(s.T(), err, &ErrQuotaLessThenZero)
	// Test batch decrement with one going below zero
	items = []cache.IncrItem{
		{BizID: 6001, Channel: domain.ChannelSMS, Val: 60}, // This will go below zero
		{BizID: 6002, Channel: domain.ChannelSMS, Val: 50},
		{BizID: 6003, Channel: domain.ChannelSMS, Val: 75},
	}
	err = s.cache.MutiDecr(s.T().Context(), items)
	// Verify that no quotas were modified (atomic operation)
	storedQuota, err := s.cache.Find(s.T().Context(), 6001, domain.ChannelSMS)
	s.NoError(err)
	s.Equal(int32(40), storedQuota.Quota)

	storedQuota, err = s.cache.Find(s.T().Context(), 6002, domain.ChannelSMS)
	s.NoError(err)
	s.Equal(int32(150), storedQuota.Quota)

	storedQuota, err = s.cache.Find(s.T().Context(), 6003, domain.ChannelSMS)
	s.NoError(err)
	s.Equal(int32(225), storedQuota.Quota)

	// Test batch decrement with empty items
	err = s.cache.MutiDecr(s.T().Context(), []cache.IncrItem{})
	s.NoError(err)
}

func TestQuotaCache(t *testing.T) {
	suite.Run(t, new(QuotaCacheTestSuite))
}
