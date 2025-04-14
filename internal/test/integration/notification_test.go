//go:build e2e

package integration

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"

	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	notificationioc "gitee.com/flycash/notification-platform/internal/test/integration/ioc/notification"
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
	db        *egorm.Component
	svc       notificationsvc.Service
	repo      repository.NotificationRepository
	quotaRepo repository.QuotaRepository
}

func (s *NotificationServiceTestSuite) SetupSuite() {
	s.db = testioc.InitDBAndTables()
	svc := notificationioc.Init()
	s.svc, s.repo, s.quotaRepo = svc.Svc, svc.Repo, svc.QuotaRepo
}

func (s *NotificationServiceTestSuite) TearDownTest() {
	// 每个测试后清空表数据
	s.db.Exec("TRUNCATE TABLE `notifications`")
	s.db.Exec("TRUNCATE TABLE `quotas`")
}

// 创建测试用的通知对象
func (s *NotificationServiceTestSuite) createTestNotification(bizID int64) domain.Notification {
	now := time.Now()
	return domain.Notification{
		BizID:     bizID,
		Key:       fmt.Sprintf("test-key-%d-%d", now.Unix(), rand.Int()),
		Receivers: []string{"13800138000"},
		Channel:   domain.ChannelSMS,
		Template: domain.Template{
			ID:        100,
			VersionID: 1,
			Params:    map[string]string{"code": "123456"},
		},
		ScheduledSTime: now,
		ScheduledETime: now.Add(1 * time.Hour),
		Status:         domain.SendStatusPending,
	}
}

func (s *NotificationServiceTestSuite) TestRepositoryCreate() {
	t := s.T()

	bizID := int64(2)
	tests := []struct {
		name          string
		before        func(t *testing.T, notification domain.Notification)
		after         func(t *testing.T, expected, actual domain.Notification)
		notification  domain.Notification
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name: "创建成功",
			before: func(t *testing.T, notification domain.Notification) {
				t.Helper()
				err := s.quotaRepo.CreateOrUpdate(t.Context(), domain.Quota{
					BizID:   notification.BizID,
					Quota:   100,
					Channel: notification.Channel,
				})
				require.NoError(t, err)
			},
			notification: func() domain.Notification {
				return s.createTestNotification(1)
			}(),
			assertErrFunc: assert.NoError,
			after: func(t *testing.T, expected, actual domain.Notification) {
				t.Helper()
				s.assertNotification(t, expected, actual)
			},
		},
		{
			name: "BizID和Key组成的唯一索引冲突",
			before: func(t *testing.T, notification domain.Notification) {
				t.Helper()
				t.Helper()

				err := s.quotaRepo.CreateOrUpdate(t.Context(), domain.Quota{
					BizID:   notification.BizID,
					Quota:   100,
					Channel: notification.Channel,
				})
				require.NoError(t, err)

				_, err = s.repo.Create(t.Context(), notification)
				assert.NoError(t, err)
			},
			notification: func() domain.Notification {
				return s.createTestNotification(bizID)
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, errs.ErrNotificationDuplicate)
			},
			after: func(t *testing.T, expected, actual domain.Notification) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t, tt.notification)

			created, err := s.repo.Create(t.Context(), tt.notification)
			tt.assertErrFunc(t, err)

			if err != nil {
				return
			}

			tt.after(t, tt.notification, created)
		})
	}
}

func (s *NotificationServiceTestSuite) assertNotification(t *testing.T, expected domain.Notification, actual domain.Notification) {
	assert.NotZero(t, actual.ID)
	assert.Equal(t, expected.BizID, actual.BizID)
	assert.Equal(t, expected.Key, actual.Key)
	assert.Equal(t, expected.Receivers, actual.Receivers)
	assert.Equal(t, expected.Channel, actual.Channel)
	assert.Equal(t, expected.Template, actual.Template)
	assert.Equal(t, expected.ScheduledSTime.UnixMilli(), actual.ScheduledSTime.UnixMilli())
	assert.Equal(t, expected.ScheduledETime.UnixMilli(), actual.ScheduledETime.UnixMilli())
	assert.Equal(t, expected.Status, actual.Status)
}

func (s *NotificationServiceTestSuite) TestRepositoryBatchCreate() {
	t := s.T()
	bizID := int64(4)

	tests := []struct {
		name          string
		notifications []domain.Notification
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name: "创建成功",
			notifications: []domain.Notification{
				s.createTestNotification(bizID),
				s.createTestNotification(bizID),
				s.createTestNotification(bizID),
			},
			assertErrFunc: assert.NoError,
		},
		{
			name: "BizID和Key组成的唯一索引冲突",
			notifications: func() []domain.Notification {
				ns := make([]domain.Notification, 2)
				ns[0] = s.createTestNotification(bizID)
				ns[1] = ns[0]
				return ns
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, errs.ErrNotificationDuplicate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := s.repo.BatchCreate(t.Context(), tt.notifications)
			tt.assertErrFunc(t, err)
			if err != nil {
				return
			}
			for i := range tt.notifications {
				s.assertNotification(t, tt.notifications[i], created[i])
			}
		})
	}
}

func (s *NotificationServiceTestSuite) TestRepositoryCASStatus() {
	t := s.T()

	bizID := int64(8)
	tests := []struct {
		name   string
		before func(t *testing.T) (uint64, int) // 返回ID和Version
		after  func(t *testing.T, id uint64)

		requireFunc require.ErrorAssertionFunc
	}{
		{
			name: "id存在，更新成功",
			before: func(t *testing.T) (uint64, int) {
				t.Helper()

				notification := s.createTestNotification(bizID)
				err := s.quotaRepo.CreateOrUpdate(t.Context(), domain.Quota{
					BizID:   bizID,
					Quota:   100,
					Channel: notification.Channel,
				})
				require.NoError(t, err)

				created, err := s.repo.Create(t.Context(), notification)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, created.Status)
				return created.ID, created.Version
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
				// 验证状态已更新
				updated, err := s.repo.GetByID(t.Context(), id)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
				assert.Equal(t, 2, updated.Version) // 版本号应该加1

				find, err := s.quotaRepo.Find(t.Context(), bizID, updated.Channel)
				require.NoError(t, err)
				assert.Equal(t, int32(99), find.Quota)
			},
			requireFunc: require.NoError,
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

			err := s.repo.CASStatus(t.Context(), domain.Notification{
				ID:      id,
				Status:  domain.SendStatusSucceeded,
				Version: version,
			})
			tt.requireFunc(t, err)

			tt.after(t, id)
		})
	}
}

func (s *NotificationServiceTestSuite) TestRepositoryUpdateStatus() {
	t := s.T()

	bizID := int64(8)
	tests := []struct {
		name   string
		before func(t *testing.T) (uint64, int) // 返回ID和Version
		after  func(t *testing.T, id uint64)

		requireFunc require.ErrorAssertionFunc
	}{
		{
			name: "id存在更新成功",
			before: func(t *testing.T) (uint64, int) {
				t.Helper()
				created, err := s.repo.Create(t.Context(), s.createTestNotification(bizID))
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, created.Status)

				return created.ID, created.Version
			},
			after: func(t *testing.T, id uint64) {
				t.Helper()
				// 验证状态已更新
				updated, err := s.repo.GetByID(t.Context(), id)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
				assert.Equal(t, 2, updated.Version) // 版本号应该加1
			},
			requireFunc: require.NoError,
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
			requireFunc: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, version := tt.before(t)

			err := s.repo.UpdateStatus(t.Context(), domain.Notification{
				ID:      id,
				Status:  domain.SendStatusSucceeded,
				Version: version,
			})
			tt.requireFunc(t, err)

			tt.after(t, id)
		})
	}
}

func (s *NotificationServiceTestSuite) TestRepositoryBatchUpdateStatusSucceededOrFailed() {
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
	createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
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
					updated, err := s.repo.GetByID(ctx, id)
					require.NoError(t, err)
					assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
					assert.Greater(t, updated.Version, initialVersions[id])
				}

				// 验证其他通知状态未变
				unchanged, err := s.repo.GetByID(ctx, createdNotifications[2].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusPending, unchanged.Status)
				assert.Equal(t, initialVersions[createdNotifications[2].ID], unchanged.Version)
			},
		},
		{
			name: "更新成功状态同时更新重试次数",
			succeededNotifications: []domain.Notification{
				{
					ID:      createdNotifications[6].ID,
					Version: initialVersions[createdNotifications[6].ID],
				},
			},
			failedNotifications: nil,
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证成功状态和重试次数更新
				updated, err := s.repo.GetByID(ctx, createdNotifications[6].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated.Status)
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
				updated, err := s.repo.GetByID(ctx, createdNotifications[2].ID)
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
					ID:      createdNotifications[3].ID,
					Version: initialVersions[createdNotifications[3].ID],
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证失败状态和重试次数更新
				updated, err := s.repo.GetByID(ctx, createdNotifications[3].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated.Status)
				assert.Greater(t, updated.Version, initialVersions[createdNotifications[3].ID])
			},
		},
		{
			name: "更新成功状态和失败状态的组合",
			succeededNotifications: []domain.Notification{
				{
					ID:      createdNotifications[4].ID,
					Version: initialVersions[createdNotifications[4].ID],
				},
			},
			failedNotifications: []domain.Notification{
				{
					ID:      createdNotifications[5].ID,
					Version: initialVersions[createdNotifications[5].ID],
				},
			},
			assertFunc: func(t *testing.T, err error, initialVersions map[uint64]int) {
				require.NoError(t, err)

				// 验证成功状态更新
				updated1, err := s.repo.GetByID(ctx, createdNotifications[4].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusSucceeded, updated1.Status)
				assert.Greater(t, updated1.Version, initialVersions[createdNotifications[4].ID])

				// 验证失败状态和重试次数更新
				updated2, err := s.repo.GetByID(ctx, createdNotifications[5].ID)
				require.NoError(t, err)
				assert.Equal(t, domain.SendStatusFailed, updated2.Status)
				assert.Greater(t, updated2.Version, initialVersions[createdNotifications[5].ID])
			},
		},
	}

	// 执行测试场景
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 更新状态
			err1 := s.repo.BatchUpdateStatusSucceededOrFailed(ctx, tt.succeededNotifications, tt.failedNotifications)

			// 断言结果
			tt.assertFunc(t, err1, initialVersions)
		})
	}
}

func (s *NotificationServiceTestSuite) TestGetByKeys() {
	t := s.T()

	bizID := int64(7)
	notifications, err := s.repo.BatchCreate(t.Context(), []domain.Notification{
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
		s.createTestNotification(bizID),
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		bizID         int64
		keysFunc      func() []string
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name:  "正常流程",
			bizID: bizID,
			keysFunc: func() []string {
				// 测试获取通知列表
				return []string{notifications[1].Key, notifications[0].Key, notifications[2].Key}
			},
			assertErrFunc: assert.NoError,
		},
		{
			name:  "keys为空",
			bizID: 1001,
			keysFunc: func() []string {
				return nil
			},
			assertErrFunc: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, errs.ErrInvalidParameter)
			},
		},
		{
			name:  "不存在的key",
			bizID: 1001,
			keysFunc: func() []string {
				return []string{"non-existent-key"}
			},
			assertErrFunc: assert.NoError,
		},
		{
			name:  "不存在的业务ID",
			bizID: 999999,
			keysFunc: func() []string {
				return []string{"test-key"}
			},
			assertErrFunc: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			found, err1 := s.svc.GetByKeys(t.Context(), tt.bizID, tt.keysFunc()...)
			tt.assertErrFunc(t, err1)

			if err1 != nil {
				return
			}

			mp := make(map[int64]domain.Notification, len(notifications))
			for i := range notifications {
				mp[notifications[i].BizID] = notifications[i]
			}

			for j := range found {
				s.assertNotification(t, mp[found[j].BizID], found[j])
			}
		})
	}
}
