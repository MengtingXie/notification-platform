//go:build e2e

package integration

import (
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/notification/internal/integration/startup"
	"math/rand"
	"testing"
	"time"

	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
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
	svc domainService
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
		Status:         domain.SendStatusPending,
	}
}

func (s *NotificationServiceTestSuite) TestCreateNotification() {
	t := s.T()
	notification := s.createTestNotification(1)

	created, err := s.svc.Create(t.Context(), notification)
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
	bizID := int64(2)
	tests := []struct {
		name          string
		before        func(t *testing.T, notification domain.Notification)
		notification  domain.Notification
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name:   "Key 为空",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Key = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "Receiver 为空",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Receiver = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "Channel 为空",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Channel = ""
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "Template.ID 小于等于0",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Template.ID = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "Template.VersionID 小于等于0",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Template.VersionID = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "Template.Params 为空",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.Template.Params = nil
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "ScheduledSTime 为0",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.ScheduledSTime = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "ScheduledETime 为0",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.ScheduledETime = 0
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:   "ScheduledETime 小于 ScheduledSTime",
			before: func(t *testing.T, notification domain.Notification) {},
			notification: func() domain.Notification {
				n := s.createTestNotification(bizID)
				n.ScheduledETime = n.ScheduledSTime - 1
				return n
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name: "BizID和Key组成的唯一索引冲突",
			before: func(t *testing.T, notification domain.Notification) {
				t.Helper()
				_, err := s.svc.Create(t.Context(), notification)
				assert.NoError(t, err)
			},
			notification: func() domain.Notification {
				return s.createTestNotification(bizID)
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrNotificationDuplicate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t, tt.notification)

			_, err := s.svc.Create(t.Context(), tt.notification)

			tt.assertErrFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchCreate() {
	t := s.T()

	bizID := int64(3)
	notifications := []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	}

	created, err := s.svc.BatchCreate(t.Context(), notifications)
	require.NoError(t, err)

	assert.ElementsMatch(t, notifications, created)
}

func (s *NotificationServiceTestSuite) TestBatchCreateFailed() {
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
		{
			name: "BizID和Key组成的唯一索引冲突",
			notifications: func() []domain.Notification {
				ns := make([]domain.Notification, 2)
				ns[0] = s.createTestNotification(bizID)
				ns[1] = ns[0]
				return ns
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, notificationsvc.ErrNotificationDuplicate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.svc.BatchCreate(t.Context(), tt.notifications)
			assert.Error(t, err)
			tt.assertErrFunc(t, err)
		})
	}
}

func (s *NotificationServiceTestSuite) TestGetByID() {
	t := s.T()
	notification := s.createTestNotification(5)

	// 先创建通知
	created, err := s.svc.Create(t.Context(), notification)
	require.NoError(t, err)

	// 测试获取通知
	found, err := s.svc.GetByID(t.Context(), created.ID)
	require.NoError(t, err)

	s.assertNotification(t, notification, found)
}

func (s *NotificationServiceTestSuite) TestGetByIDFailed() {
	t := s.T()
	_, err := s.svc.GetByID(t.Context(), 999999)
	assert.ErrorIs(t, err, notificationsvc.ErrNotificationNotFound)
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
	createdNotifications, err := s.svc.BatchCreate(ctx, notifications)
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

func (s *NotificationServiceTestSuite) TestGetByBizID() {
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
				notifications, err := s.svc.BatchCreate(t.Context(), []domain.Notification{
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

			actual, err := s.svc.GetByBizID(t.Context(), tt.bizID)
			require.NoError(t, err)

			tt.assertFunc(t, expected, actual)
		})
	}
}

func (s *NotificationServiceTestSuite) TestGetByKeys() {
	t := s.T()

	// 先创建通知
	bizID := int64(7)
	notifications, err := s.svc.BatchCreate(t.Context(), []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	})
	require.NoError(t, err)

	// 测试获取通知列表
	keys := []string{notifications[1].Key, notifications[0].Key, notifications[2].Key}
	found, err := s.svc.GetByKeys(t.Context(), bizID, keys...)
	require.NoError(t, err)

	assert.ElementsMatch(t, notifications, found)
}

func (s *NotificationServiceTestSuite) TestGetByKeysFailed() {
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
			found, err := s.svc.GetByKeys(t.Context(), tt.bizID, tt.keys...)
			tt.assertErrFunc(t, err)
			if err == nil {
				assert.Empty(t, found)
			}
		})
	}
}

func (s *NotificationServiceTestSuite) TestUpdateStatus() {
	t := s.T()

	bizID := int64(8)
	tests := []struct {
		name   string
		before func(t *testing.T) (uint64, int) // 返回ID和Version
		after  func(t *testing.T, id uint64)

		requireFunc require.ErrorAssertionFunc
	}{
		{
			name: "id存在，并发更新成功",
			before: func(t *testing.T) (uint64, int) {
				t.Helper()
				created, err := s.svc.Create(t.Context(), s.createTestNotification(bizID))
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, created.Status)

				return created.ID, created.Version
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
				// 验证状态已更新
				updated, err := s.svc.GetByID(t.Context(), id)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
				assert.Equal(t, 2, updated.Version) // 版本号应该加1
			},
			requireFunc: require.NoError,
		},
		{
			name: "id存在，并发更新失败",
			before: func(t *testing.T) (uint64, int) {
				t.Helper()
				// 创建一条记录
				created, err := s.svc.Create(t.Context(), s.createTestNotification(bizID))
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, created.Status)

				// 模拟其他协程修改记录为发送失败
				err = s.svc.UpdateStatus(t.Context(), created.ID, domain.SendStatusFailed, created.Version)
				assert.NoError(t, err)

				return created.ID, created.Version
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
				// 验证状态已更新，其他协程修改成功
				updated, err := s.svc.GetByID(t.Context(), id)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated.Status)
				assert.Equal(t, 2, updated.Version)
			},
			requireFunc: require.Error,
		},
		{
			name: "id不存在",
			before: func(t *testing.T) (uint64, int) {
				t.Helper()
				return 999999, 1
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
			},
			requireFunc: require.Error, // 应该返回错误
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, version := tt.before(t)

			err := s.svc.UpdateStatus(t.Context(), id, domain.SendStatusSucceeded, version)
			tt.requireFunc(t, err)

			tt.after(t, id)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchUpdateStatusSucceededOrFailed() {
	t := s.T()
	ctx := t.Context()

	// 准备测试数据 - 创建多条通知记录
	bizID := int64(10)
	notifications := []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	}

	// 批量创建通知记录
	createdNotifications, err := s.svc.BatchCreate(ctx, notifications)
	require.NoError(t, err)
	require.Len(t, createdNotifications, len(notifications))

	// 记录初始版本号
	initialVersions := make(map[uint64]int)
	for _, n := range createdNotifications {
		initialVersions[n.ID] = n.Version
	}

	// 定义测试场景
	tests := []struct {
		name                   string
		succeededNotifications []domain.Notification // 要更新为成功状态的通知
		failedNotifications    []domain.Notification // 要更新为失败状态的通知
		assertFunc             func(t *testing.T, err error, initialVersions map[uint64]int)
	}{
		{
			name: "仅更新成功状态但不更新重试次数",
			succeededNotifications: []domain.Notification{
				{
					ID:      createdNotifications[0].ID,
					Version: initialVersions[createdNotifications[0].ID],
				},
				{
					ID:      createdNotifications[1].ID,
					Version: initialVersions[createdNotifications[1].ID],
				},
			},
			failedNotifications: nil,
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证成功状态更新
				for _, id := range []uint64{createdNotifications[0].ID, createdNotifications[1].ID} {
					updated, err := s.svc.GetByID(ctx, id)
					require.NoError(t, err)
					assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
					assert.Greater(t, updated.Version, initialVersions[id])
				}

				// 验证其他通知状态未变
				unchanged, err := s.svc.GetByID(ctx, createdNotifications[2].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, unchanged.Status)
				assert.Equal(t, initialVersions[createdNotifications[2].ID], unchanged.Version)
			},
		},
		{
			name: "更新成功状态同时更新重试次数",
			succeededNotifications: []domain.Notification{
				{
					ID:         createdNotifications[6].ID,
					RetryCount: 5,
					Version:    initialVersions[createdNotifications[6].ID],
				},
			},
			failedNotifications: nil,
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证成功状态和重试次数更新
				updated, err := s.svc.GetByID(ctx, createdNotifications[6].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
				assert.Equal(t, int8(5), updated.RetryCount) // 验证重试次数已更新
				assert.Greater(t, updated.Version, initialVersions[createdNotifications[6].ID])
			},
		},
		{
			name:                   "仅更新失败状态但不更新重试次数",
			succeededNotifications: nil,
			failedNotifications: []domain.Notification{
				{
					ID:      createdNotifications[2].ID,
					Version: initialVersions[createdNotifications[2].ID],
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证失败状态更新
				updated, err := s.svc.GetByID(ctx, createdNotifications[2].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated.Status)
				assert.Greater(t, updated.Version, initialVersions[createdNotifications[2].ID])
			},
		},
		{
			name:                   "更新失败状态同时更新重试次数",
			succeededNotifications: nil,
			failedNotifications: []domain.Notification{
				{
					ID:         createdNotifications[3].ID,
					RetryCount: 2,
					Version:    initialVersions[createdNotifications[3].ID],
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证失败状态和重试次数更新
				updated, err := s.svc.GetByID(ctx, createdNotifications[3].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated.Status)
				assert.Equal(t, int8(2), updated.RetryCount)
				assert.Greater(t, updated.Version, initialVersions[createdNotifications[3].ID])
			},
		},
		{
			name: "更新成功状态和失败状态的组合",
			succeededNotifications: []domain.Notification{
				{
					ID:         createdNotifications[4].ID,
					RetryCount: 4,
					Version:    initialVersions[createdNotifications[4].ID],
				},
			},
			failedNotifications: []domain.Notification{
				{
					ID:         createdNotifications[5].ID,
					RetryCount: 3,
					Version:    initialVersions[createdNotifications[5].ID],
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证成功状态更新
				updated1, err := s.svc.GetByID(ctx, createdNotifications[4].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated1.Status)
				assert.Equal(t, int8(4), updated1.RetryCount)
				assert.Greater(t, updated1.Version, initialVersions[createdNotifications[4].ID])

				// 验证失败状态和重试次数更新
				updated2, err := s.svc.GetByID(ctx, createdNotifications[5].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated2.Status)
				assert.Equal(t, int8(3), updated2.RetryCount)
				assert.Greater(t, updated2.Version, initialVersions[createdNotifications[5].ID])
			},
		},
		{
			name:                   "空的参数列表",
			succeededNotifications: []domain.Notification{},
			failedNotifications:    []domain.Notification{},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, notificationsvc.ErrInvalidParameter)
			},
		},
		{
			name:                   "版本号不匹配的情况",
			succeededNotifications: nil,
			failedNotifications: []domain.Notification{
				{
					ID:      createdNotifications[0].ID,
					Version: 999, // 错误的版本号
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				assert.Error(t, err)
				// 状态应该保持不变
				unchanged, err := s.svc.GetByID(ctx, createdNotifications[0].ID)
				require.NoError(t, err)
				// 此时应该是成功状态，因为前面的测试已经更新过了
				assert.Equal(t, domain.SendStatusSucceeded, unchanged.Status)
				// 版本号应该是前面测试增加后的值，而不是999
				assert.NotEqual(t, 999, unchanged.Version)
			},
		},
	}

	// 执行测试场景
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 更新状态
			err := s.svc.BatchUpdateStatusSucceededOrFailed(ctx, tt.succeededNotifications, tt.failedNotifications)

			// 断言结果
			tt.assertFunc(t, err, initialVersions)
		})
	}
}

func (s *NotificationServiceTestSuite) TestBatchUpdateStatus() {
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
		created, err := s.svc.Create(ctx, notification)
		require.NoError(t, err)
		createdNotifications[i] = created
	}

	// 选择前两个通知进行状态更新
	ids := []uint64{createdNotifications[0].ID, createdNotifications[1].ID}
	newStatus := domain.SendStatusSucceeded

	// 测试批量更新状态
	err := s.svc.BatchUpdateStatus(ctx, ids, newStatus)
	require.NoError(t, err)

	// 验证已更新的通知状态
	for i := range ids {
		updated, err := s.svc.GetByID(ctx, ids[i])
		require.NoError(t, err)
		assert.Equal(t, newStatus, updated.Status)
		assert.Greater(t, updated.Version, createdNotifications[i].Version)
	}

	// 验证未更新的通知状态未变
	unaffected, err := s.svc.GetByID(ctx, createdNotifications[2].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SendStatusPending, unaffected.Status)
	assert.Equal(t, unaffected.Version, createdNotifications[2].Version)
}
