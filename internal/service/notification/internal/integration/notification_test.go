//go:build e2e

package integration

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/integration/startup"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/service"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestNotificationServiceSuite(t *testing.T) {
	suite.Run(t, new(NotificationServiceTestSuite))
}

type NotificationServiceTestSuite struct {
	suite.Suite
	db  *egorm.Component
	svc service.NotificationService
}

func (s *NotificationServiceTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	s.svc = startup.InitNotificationService()
}

func (s *NotificationServiceTestSuite) TearDownTest() {
	// 每个测试后清空表数据
	s.db.Exec("TRUNCATE TABLE `notifications`")
}

// 创建测试用的通知对象
func (s *NotificationServiceTestSuite) createTestNotification(bizID int64) domain.Notification {
	now := time.Now()
	return domain.Notification{
		BizID:    bizID,
		Key:      fmt.Sprintf("test-key-%d-%d", now.Unix(), rand.Int()),
		Receiver: "13800138000",
		Channel:  domain.ChannelSMS,
		Template: domain.Template{
			ID:        100,
			VersionID: 1,
			Params:    map[string]string{"code": "123456"},
		},
		ScheduledSTime: now.Unix(),
		ScheduledETime: now.Add(1 * time.Hour).Unix(),
		Status:         domain.StatusPending,
	}
}

func (s *NotificationServiceTestSuite) TestCreateNotification() {
	t := s.T()
	notification := s.createTestNotification(1)

	created, err := s.svc.CreateNotification(t.Context(), notification)
	require.NoError(t, err)

	s.assertNotification(t, notification, created)
}

func (s *NotificationServiceTestSuite) assertNotification(t *testing.T, expected domain.Notification, actual domain.Notification) {
	assert.NotZero(t, actual.ID)
	assert.Equal(t, expected.BizID, actual.BizID)
	assert.Equal(t, expected.Key, actual.Key)
	assert.Equal(t, expected.Receiver, actual.Receiver)
	assert.Equal(t, expected.Channel, actual.Channel)
	assert.Equal(t, expected.Template, actual.Template)
	assert.Equal(t, expected.ScheduledSTime, actual.ScheduledSTime)
	assert.Equal(t, expected.ScheduledETime, actual.ScheduledETime)
	assert.Equal(t, expected.Status, actual.Status)
}

func (s *NotificationServiceTestSuite) TestCreateNotificationFailed() {
	t := s.T()
	tests := []struct {
		name          string
		notification  domain.Notification
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name: "Key 为空",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Key = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "Receiver 为空",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Receiver = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "Channel 为空",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Channel = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "Template.ID 小于等于0",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Template.ID = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "Template.VersionID 小于等于0",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Template.VersionID = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "Template.Params 为空",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.Template.Params = nil
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "ScheduledSTime 为0",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.ScheduledSTime = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "ScheduledETime 为0",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.ScheduledETime = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "ScheduledETime 小于 ScheduledSTime",
			notification: func() domain.Notification {
				n := s.createTestNotification(2)
				n.ScheduledETime = n.ScheduledSTime - 1
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.svc.CreateNotification(t.Context(), tt.notification)
			assert.Error(t, err)
			tt.assertErrFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchCreateNotifications() {
	t := s.T()

	bizID := int64(3)
	notifications := []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	}

	created, err := s.svc.BatchCreateNotifications(t.Context(), notifications)
	require.NoError(t, err)

	assert.ElementsMatch(t, notifications, created)
}

func (s *NotificationServiceTestSuite) TestBatchCreateNotificationsFailed() {
	t := s.T()
	bizID := int64(4)

	tests := []struct {
		name          string
		notifications []domain.Notification
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name:          "通知列表为空",
			notifications: nil,
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "存在无效的通知",
			notifications: []domain.Notification{
				s.createTestNotification(bizID),
				func() domain.Notification {
					n := s.createTestNotification(bizID)
					n.Key = ""
					return n
				}(),
				s.createTestNotification(bizID),
			},
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.svc.BatchCreateNotifications(t.Context(), tt.notifications)
			assert.Error(t, err)
			tt.assertErrFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestGetNotificationByID() {
	t := s.T()
	notification := s.createTestNotification(5)

	// 先创建通知
	created, err := s.svc.CreateNotification(t.Context(), notification)
	require.NoError(t, err)

	// 测试获取通知
	found, err := s.svc.GetNotificationByID(t.Context(), created.ID)
	require.NoError(t, err)

	s.assertNotification(t, notification, found)
}

func (s *NotificationServiceTestSuite) TestGetNotificationByIDFailed() {
	t := s.T()
	_, err := s.svc.GetNotificationByID(t.Context(), 999999)
	assert.ErrorIs(t, err, service.ErrNotificationNotFound)
}

func (s *NotificationServiceTestSuite) TestGetNotificationsByBizID() {
	t := s.T()

	tests := []struct {
		name  string
		bizID int64

		before     func(t *testing.T, bizID int64) []domain.Notification
		assertFunc assert.ComparisonAssertionFunc
	}{
		{
			name:  "bizID存在",
			bizID: int64(6),
			before: func(t *testing.T, bizID int64) []domain.Notification {
				t.Helper()
				// 先创建通知
				notifications, err := s.svc.BatchCreateNotifications(t.Context(), []domain.Notification{
					s.createTestNotification(bizID),
					s.createTestNotification(bizID),
					s.createTestNotification(bizID),
				})
				require.NoError(t, err)
				return notifications
			},
			assertFunc: assert.ElementsMatch,
		},
		{
			name:  "bizID不存在",
			bizID: int64(999999),
			before: func(t *testing.T, bizID int64) []domain.Notification {
				t.Helper()
				return nil
			},
			assertFunc: assert.ElementsMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := tt.before(t, tt.bizID)

			actual, err := s.svc.GetNotificationsByBizID(t.Context(), tt.bizID)
			require.NoError(t, err)

			tt.assertFunc(t, expected, actual)
		})
	}
}

func (s *NotificationServiceTestSuite) TestGetNotificationsByKeys() {
	t := s.T()

	// 先创建通知
	bizID := int64(7)
	notifications, err := s.svc.BatchCreateNotifications(t.Context(), []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	})
	require.NoError(t, err)

	// 测试获取通知列表
	keys := []string{notifications[1].Key, notifications[0].Key, notifications[2].Key}
	found, err := s.svc.GetNotificationsByKeys(t.Context(), bizID, keys...)
	require.NoError(t, err)

	assert.ElementsMatch(t, notifications, found)
}

func (s *NotificationServiceTestSuite) TestGetNotificationsByKeysFailed() {
	t := s.T()

	tests := []struct {
		name          string
		bizID         int64
		keys          []string
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name:  "keys为空",
			bizID: 1001,
			keys:  nil,
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:          "不存在的key",
			bizID:         1001,
			keys:          []string{"non-existent-key"},
			assertErrFunc: assert.NoError,
		},
		{
			name:          "不存在的业务ID",
			bizID:         999999,
			keys:          []string{"test-key"},
			assertErrFunc: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := s.svc.GetNotificationsByKeys(t.Context(), tt.bizID, tt.keys...)
			tt.assertErrFunc(t, err)
			if err == nil {
				assert.Empty(t, found)
			}
		})
	}
}

func (s *NotificationServiceTestSuite) TestUpdateNotificationStatus() {
	t := s.T()

	tests := []struct {
		name   string
		before func(t *testing.T) uint64
		after  func(t *testing.T, id uint64)

		requireFunc require.ErrorAssertionFunc
	}{
		{
			name: "id存在",
			before: func(t *testing.T) uint64 {
				t.Helper()

				// 先创建通知
				created, err := s.svc.CreateNotification(t.Context(), s.createTestNotification(int64(8)))
				require.NoError(t, err)
				assert.Equal(t, domain.StatusPending, created.Status)

				return created.ID
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
				// 验证状态已更新
				updated, err := s.svc.GetNotificationByID(t.Context(), id)
				require.NoError(t, err)
				assert.Equal(t, domain.StatusSucceeded, updated.Status)
			},
			requireFunc: require.NoError,
		},
		{
			name: "id不存在",
			before: func(t *testing.T) uint64 {
				t.Helper()
				return 999999
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
			},
			requireFunc: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := tt.before(t)

			err := s.svc.UpdateNotificationStatus(t.Context(), id, domain.StatusSucceeded)
			tt.requireFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchUpdateNotificationStatusSucceededOrFailed() {
	t := s.T()

	// 批量创建通知记录
	bizID := int64(10)
	ns, err := s.svc.BatchCreateNotifications(t.Context(), []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	})
	require.NoError(t, err)

	// 批量更新状态
	succeeded := []domain.Notification{ns[0], ns[1]}
	failed := []domain.Notification{ns[2]}
	err = s.svc.BatchUpdateNotificationStatusSucceededOrFailed(t.Context(), succeeded, failed)
	require.NoError(t, err)

	// 验证状态已更新
	for _, n := range succeeded {
		updated, err := s.svc.GetNotificationByID(t.Context(), n.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusSucceeded, updated.Status)
	}
	for _, n := range failed {
		updated, err := s.svc.GetNotificationByID(t.Context(), n.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusFailed, updated.Status)
	}
}

func (s *NotificationServiceTestSuite) TestBatchUpdateNotificationStatusSucceededOrFailedFailed() {
	t := s.T()

	tests := []struct {
		name                   string
		succeededNotifications []domain.Notification
		failedNotifications    []domain.Notification
		assertErrFunc          assert.ErrorAssertionFunc
	}{
		{
			name:                   "成功和失败的通知列表都为空",
			succeededNotifications: nil,
			failedNotifications:    nil,
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.svc.BatchUpdateNotificationStatusSucceededOrFailed(t.Context(), tt.succeededNotifications, tt.failedNotifications)
			assert.Error(t, err)
			tt.assertErrFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchUpdateNotificationStatus() {
	t := s.T()
	ctx := t.Context()

	// 准备测试数据 - 创建多条通知记录
	notifications := []domain.Notification{
		s.createTestNotification(1),
		s.createTestNotification(2),
		s.createTestNotification(3),
	}

	// 批量创建通知记录
	createdNotifications := make([]domain.Notification, len(notifications))
	for i, notification := range notifications {
		created, err := s.svc.CreateNotification(ctx, notification)
		require.NoError(t, err)
		createdNotifications[i] = created
	}

	// 选择前两个通知进行状态更新
	ids := []uint64{createdNotifications[0].ID, createdNotifications[1].ID}
	newStatus := domain.StatusSucceeded

	// 测试批量更新状态
	err := s.svc.BatchUpdateNotificationStatus(ctx, ids, string(newStatus))
	require.NoError(t, err)

	// 验证已更新的通知状态
	for _, id := range ids {
		updated, err := s.svc.GetNotificationByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, newStatus, updated.Status)
	}

	// 验证未更新的通知状态未变
	unaffected, err := s.svc.GetNotificationByID(ctx, createdNotifications[2].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, unaffected.Status)
}

func (s *NotificationServiceTestSuite) TestBatchGetByIDs() {
	t := s.T()
	ctx := t.Context()

	// 创建多条通知记录用于测试
	notifications := []domain.Notification{
		s.createTestNotification(100),
		s.createTestNotification(101),
		s.createTestNotification(102),
	}

	// 批量创建通知记录
	createdNotifications, err := s.svc.BatchCreateNotifications(ctx, notifications)
	require.NoError(t, err)
	require.Len(t, createdNotifications, len(notifications))

	// 提取通知ID
	var ids []uint64
	expectedMap := make(map[uint64]domain.Notification)
	for _, n := range createdNotifications {
		ids = append(ids, n.ID)
		expectedMap[n.ID] = n
	}

	tests := []struct {
		name       string
		inputIDs   []uint64
		assertFunc func(t *testing.T, result map[uint64]domain.Notification, err error)
	}{
		{
			name:     "获取所有已创建的通知",
			inputIDs: ids,
			assertFunc: func(t *testing.T, result map[uint64]domain.Notification, err error) {
				require.NoError(t, err)
				require.Len(t, result, len(ids))

				for id, notification := range result {
					expected, exists := expectedMap[id]
					assert.True(t, exists)
					s.assertNotification(t, expected, notification)
				}
			},
		},
		{
			name:     "只获取部分ID对应的通知",
			inputIDs: ids[:2], // 只取前两个ID
			assertFunc: func(t *testing.T, result map[uint64]domain.Notification, err error) {
				require.NoError(t, err)
				require.Len(t, result, 2)

				for id, notification := range result {
					expected, exists := expectedMap[id]
					assert.True(t, exists)
					s.assertNotification(t, expected, notification)
				}
			},
		},
		{
			name:     "包含不存在的ID",
			inputIDs: append([]uint64{}, ids[0], 999999), // 一个存在的ID和一个不存在的ID
			assertFunc: func(t *testing.T, result map[uint64]domain.Notification, err error) {
				require.NoError(t, err)
				require.Len(t, result, 1) // 应该只返回1个存在的通知

				// 验证返回的是正确的通知
				notification, exists := result[ids[0]]
				assert.True(t, exists)
				s.assertNotification(t, expectedMap[ids[0]], notification)

				// 验证不存在的ID没有返回结果
				_, exists = result[999999]
				assert.False(t, exists)
			},
		},
		{
			name:     "空ID列表",
			inputIDs: []uint64{},
			assertFunc: func(t *testing.T, result map[uint64]domain.Notification, err error) {
				require.NoError(t, err)
				assert.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.svc.BatchGetByIDs(ctx, tt.inputIDs)
			tt.assertFunc(t, result, err)
		})
	}
}
