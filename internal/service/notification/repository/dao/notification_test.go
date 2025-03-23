//go:build e2e

package dao

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
)

const (
	notificationChannelEmail = "EMAIL"
)

func TestNotificationDAOSuite(t *testing.T) {
	suite.Run(t, new(NotificationDAOTestSuite))
}

type NotificationDAOTestSuite struct {
	suite.Suite
	db  *gorm.DB
	dao NotificationDAO
}

func (s *NotificationDAOTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	// 确保表存在并且有正确的结构
	err := s.db.AutoMigrate(&Notification{})
	s.NoError(err)
	s.dao = NewNotificationDAO(s.db)
}

func (s *NotificationDAOTestSuite) TearDownTest() {
	// 每个测试后清空表数据
	s.db.Exec("TRUNCATE TABLE `notifications`")
}

func (s *NotificationDAOTestSuite) TestCreate() {
	t := s.T()
	ctx := context.Background()

	notification := Notification{
		ID:                1,
		BizID:             123,
		Key:               "test_key_1",
		Receiver:          "user@example.com",
		Channel:           notificationChannelEmail,
		TemplateID:        101,
		TemplateVersionID: 1001,
		Status:            notificationStatusPending,
		RetryCount:        0,
		ScheduledSTime:    time.Now().Unix(),
		ScheduledETime:    time.Now().Add(time.Hour).Unix(),
	}

	err := s.dao.Create(ctx, notification)
	assert.NoError(t, err)

	// 验证创建成功
	var result Notification
	err = s.db.First(&result, "id = ?", notification.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Key, result.Key)
	assert.Equal(t, notification.Receiver, result.Receiver)
	assert.Equal(t, notification.Channel, result.Channel)
	assert.Equal(t, notification.TemplateID, result.TemplateID)
	assert.Equal(t, notification.TemplateVersionID, result.TemplateVersionID)
	assert.Equal(t, notification.Status, result.Status)
	assert.NotZero(t, result.Ctime)
	assert.NotZero(t, result.Utime)
}

func (s *NotificationDAOTestSuite) TestBatchCreate() {
	t := s.T()
	ctx := context.Background()

	notifications := []Notification{
		{
			ID:                2,
			BizID:             234,
			Key:               "test_key_2",
			Receiver:          "user1@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        102,
			TemplateVersionID: 1002,
			Status:            notificationStatusPending,
			ScheduledSTime:    time.Now().Unix(),
			ScheduledETime:    time.Now().Add(time.Hour).Unix(),
		},
		{
			ID:                3,
			BizID:             345,
			Key:               "test_key_3",
			Receiver:          "user2@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        102,
			TemplateVersionID: 1002,
			Status:            notificationStatusPending,
			ScheduledSTime:    time.Now().Unix(),
			ScheduledETime:    time.Now().Add(time.Hour).Unix(),
		},
	}

	err := s.dao.BatchCreate(ctx, notifications)
	assert.NoError(t, err)

	// 验证批量创建成功 - 完整比较每个通知的字段
	for _, expected := range notifications {
		var actual Notification
		err := s.db.First(&actual, expected.ID).Error
		assert.NoError(t, err)

		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.BizID, actual.BizID)
		assert.Equal(t, expected.Key, actual.Key)
		assert.Equal(t, expected.Receiver, actual.Receiver)
		assert.Equal(t, expected.Channel, actual.Channel)
		assert.Equal(t, expected.TemplateID, actual.TemplateID)
		assert.Equal(t, expected.TemplateVersionID, actual.TemplateVersionID)
		assert.Equal(t, expected.Status, actual.Status)
		assert.Equal(t, expected.ScheduledSTime, actual.ScheduledSTime)
		assert.Equal(t, expected.ScheduledETime, actual.ScheduledETime)
		assert.NotZero(t, actual.Ctime)
		assert.NotZero(t, actual.Utime)
	}
}

func (s *NotificationDAOTestSuite) TestUpdateStatus() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据
	now := time.Now().Add(-5 * time.Second)
	notification := Notification{
		ID:                4,
		BizID:             456,
		Key:               "test_key_4",
		Receiver:          "user@example.com",
		Channel:           notificationChannelEmail,
		TemplateID:        104,
		TemplateVersionID: 1004,
		Status:            notificationStatusPending,
		ScheduledSTime:    now.Unix(),
		ScheduledETime:    now.Add(time.Hour).Unix(),
		Ctime:             now.Unix(),
		Utime:             now.Unix(),
	}

	err := s.db.Create(&notification).Error
	assert.NoError(t, err)

	// 测试更新状态
	err = s.dao.UpdateStatus(ctx, notification.ID, notification.BizID, notificationStatusSucceeded)
	assert.NoError(t, err)

	// 验证状态已更新
	var result Notification
	err = s.db.First(&result, notification.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, notificationStatusSucceeded, result.Status)
	assert.Greater(t, result.Utime, notification.Utime)
}

func (s *NotificationDAOTestSuite) TestFindByID() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据
	notification := Notification{
		ID:                5,
		BizID:             567,
		Key:               "test_key_5",
		Receiver:          "user@example.com",
		Channel:           notificationChannelEmail,
		TemplateID:        105,
		TemplateVersionID: 1005,
		Status:            notificationStatusPending,
		ScheduledSTime:    time.Now().Unix(),
		ScheduledETime:    time.Now().Add(time.Hour).Unix(),
		Ctime:             time.Now().Unix(),
		Utime:             time.Now().Unix(),
	}

	err := s.db.Create(&notification).Error
	assert.NoError(t, err)

	// 测试查询
	result, err := s.dao.FindByID(ctx, notification.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, Notification{}, result) // 确保不是空结构体
	assert.Equal(t, notification.ID, result.ID)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Key, result.Key)

	assert.Equal(t, notification.ID, result.ID)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Key, result.Key)
	assert.Equal(t, notification.Receiver, result.Receiver)
	assert.Equal(t, notification.Channel, result.Channel)
	assert.Equal(t, notification.TemplateID, result.TemplateID)
	assert.Equal(t, notification.TemplateVersionID, result.TemplateVersionID)
	assert.Equal(t, notification.Status, result.Status)
	assert.Equal(t, notification.ScheduledSTime, result.ScheduledSTime)
	assert.Equal(t, notification.ScheduledETime, result.ScheduledETime)
	assert.Equal(t, notification.Ctime, result.Ctime)
	assert.Equal(t, notification.Utime, result.Utime)

	// 测试查询不存在记录
	result, err = s.dao.FindByID(ctx, 9999)
	assert.Error(t, err)
	assert.Equal(t, Notification{}, result) // 应返回空结构体
}

func (s *NotificationDAOTestSuite) TestFindByBizID() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据 - 同一个bizID的多条记录
	bizID := int64(678)
	notifications := []Notification{
		{
			ID:                6,
			BizID:             bizID,
			Key:               "test_key_6",
			Receiver:          "user1@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        106,
			TemplateVersionID: 1006,
			Status:            notificationStatusPending,
			ScheduledSTime:    time.Now().Unix(),
			ScheduledETime:    time.Now().Add(time.Hour).Unix(),
			Ctime:             time.Now().Unix(),
			Utime:             time.Now().Unix(),
		},
		{
			ID:                7,
			BizID:             bizID,
			Key:               "test_key_7",
			Receiver:          "user2@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        106,
			TemplateVersionID: 1006,
			Status:            notificationStatusPending,
			ScheduledSTime:    time.Now().Unix(),
			ScheduledETime:    time.Now().Add(time.Hour).Unix(),
			Ctime:             time.Now().Unix(),
			Utime:             time.Now().Unix(),
		},
	}

	err := s.db.CreateInBatches(notifications, len(notifications)).Error
	assert.NoError(t, err)

	// 测试查询
	results, err := s.dao.FindByBizID(ctx, bizID)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 创建期望的结果集合，因为返回顺序可能不确定
	resultMap := make(map[uint64]Notification)
	for _, result := range results {
		resultMap[result.ID] = result
	}

	// 验证每条记录的完整性
	for _, expected := range notifications {
		result, ok := resultMap[expected.ID]
		assert.True(t, ok, "未找到期望的记录ID: %d", expected.ID)
		assert.Equal(t, expected.BizID, result.BizID)
		assert.Equal(t, expected.Key, result.Key)
		assert.Equal(t, expected.Receiver, result.Receiver)
		assert.Equal(t, expected.Channel, result.Channel)
		assert.Equal(t, expected.TemplateID, result.TemplateID)
		assert.Equal(t, expected.TemplateVersionID, result.TemplateVersionID)
		assert.Equal(t, expected.Status, result.Status)
		assert.Equal(t, expected.ScheduledSTime, result.ScheduledSTime)
		assert.Equal(t, expected.ScheduledETime, result.ScheduledETime)
		assert.Equal(t, expected.Ctime, result.Ctime)
		assert.Equal(t, expected.Utime, result.Utime)
	}

	// 测试查询不存在记录
	results, err = s.dao.FindByBizID(ctx, 9999)
	assert.NoError(t, err)
	assert.Len(t, results, 0) // 应该返回空切片
}

func (s *NotificationDAOTestSuite) TestListByStatus() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据
	now := time.Now().Unix()
	notifications := []Notification{
		{
			ID:                9,
			BizID:             789,
			Key:               "test_key_9",
			Receiver:          "user1@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        109,
			TemplateVersionID: 1009,
			Status:            notificationStatusPending,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                10,
			BizID:             890,
			Key:               "test_key_10",
			Receiver:          "user2@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        110,
			TemplateVersionID: 1010,
			Status:            notificationStatusSucceeded,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                11,
			BizID:             901,
			Key:               "test_key_11",
			Receiver:          "user3@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        111,
			TemplateVersionID: 1011,
			Status:            notificationStatusPending,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
	}

	err := s.db.CreateInBatches(notifications, len(notifications)).Error
	assert.NoError(t, err)

	// 测试查询 - 验证PENDING状态的记录
	results, err := s.dao.ListByStatus(ctx, notificationStatusPending, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试查询 - 验证SUCCEEDED状态的记录
	results, err = s.dao.ListByStatus(ctx, notificationStatusSucceeded, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func (s *NotificationDAOTestSuite) TestListByScheduleTime() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据
	now := time.Now().Unix()
	laterTime := now + 1800
	notifications := []Notification{
		{
			ID:                12,
			BizID:             1012,
			Key:               "test_key_12",
			Receiver:          "user1@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        112,
			TemplateVersionID: 1012,
			Status:            notificationStatusPending,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                13,
			BizID:             1013,
			Key:               "test_key_13",
			Receiver:          "user2@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        113,
			TemplateVersionID: 1013,
			Status:            notificationStatusPending,
			ScheduledSTime:    laterTime,
			ScheduledETime:    laterTime + 3600,
			Ctime:             now,
			Utime:             now,
		},
	}

	err := s.db.CreateInBatches(notifications, len(notifications)).Error
	assert.NoError(t, err)

	// 测试查询近期时间窗口
	results, err := s.dao.ListByScheduleTime(ctx, now-10, now+10, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	// 验证结果的完整性
	assert.Equal(t, notifications[0].ID, results[0].ID)
	assert.Equal(t, notifications[0].BizID, results[0].BizID)
	assert.Equal(t, notifications[0].Key, results[0].Key)
	assert.Equal(t, notifications[0].Receiver, results[0].Receiver)
	assert.Equal(t, notifications[0].Channel, results[0].Channel)
	assert.Equal(t, notifications[0].TemplateID, results[0].TemplateID)
	assert.Equal(t, notifications[0].TemplateVersionID, results[0].TemplateVersionID)
	assert.Equal(t, notifications[0].Status, results[0].Status)
	assert.Equal(t, notifications[0].ScheduledSTime, results[0].ScheduledSTime)
	assert.Equal(t, notifications[0].ScheduledETime, results[0].ScheduledETime)

	// 测试查询更大时间窗口
	results, err = s.dao.ListByScheduleTime(ctx, now-10, laterTime+10, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 创建期望的结果集合，因为返回顺序可能按时间排序
	assert.Equal(t, notifications[0].ID, results[0].ID, "应该按照时间顺序返回，时间较早的记录在前")
	assert.Equal(t, notifications[1].ID, results[1].ID, "应该按照时间顺序返回，时间较晚的记录在后")

	// 验证每条记录的完整性
	for i, result := range results {
		expected := notifications[i]
		assert.Equal(t, expected.ID, result.ID)
		assert.Equal(t, expected.BizID, result.BizID)
		assert.Equal(t, expected.Key, result.Key)
		assert.Equal(t, expected.Receiver, result.Receiver)
		assert.Equal(t, expected.Channel, result.Channel)
		assert.Equal(t, expected.TemplateID, result.TemplateID)
		assert.Equal(t, expected.TemplateVersionID, result.TemplateVersionID)
		assert.Equal(t, expected.Status, result.Status)
		assert.Equal(t, expected.ScheduledSTime, result.ScheduledSTime)
		assert.Equal(t, expected.ScheduledETime, result.ScheduledETime)
	}
}

func (s *NotificationDAOTestSuite) TestBatchUpdateStatusSucceededOrFailed() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据 - 9条通知记录
	now := time.Now().Add(-30 * time.Second).Unix()
	notifications := []Notification{
		{
			ID:                201,
			BizID:             1201,
			Key:               "batch_update_key_1",
			Receiver:          "user1@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        201,
			TemplateVersionID: 2001,
			Status:            notificationStatusPending,
			RetryCount:        0,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                202,
			BizID:             1202,
			Key:               "batch_update_key_2",
			Receiver:          "user2@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        202,
			TemplateVersionID: 2002,
			Status:            notificationStatusPending,
			RetryCount:        0,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                203,
			BizID:             1203,
			Key:               "batch_update_key_3",
			Receiver:          "user3@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        203,
			TemplateVersionID: 2003,
			Status:            notificationStatusPending,
			RetryCount:        0,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                204,
			BizID:             1204,
			Key:               "batch_update_key_4",
			Receiver:          "user4@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        204,
			TemplateVersionID: 2004,
			Status:            notificationStatusPending,
			RetryCount:        1,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                205,
			BizID:             1205,
			Key:               "batch_update_key_5",
			Receiver:          "user5@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        205,
			TemplateVersionID: 2005,
			Status:            notificationStatusPending,
			RetryCount:        1,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                206,
			BizID:             1206,
			Key:               "batch_update_key_6",
			Receiver:          "user6@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        206,
			TemplateVersionID: 2006,
			Status:            notificationStatusPending,
			RetryCount:        1,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                207,
			BizID:             1207,
			Key:               "batch_update_key_7",
			Receiver:          "user7@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        207,
			TemplateVersionID: 2007,
			Status:            notificationStatusPending,
			RetryCount:        2,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                208,
			BizID:             1208,
			Key:               "batch_update_key_8",
			Receiver:          "user8@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        208,
			TemplateVersionID: 2008,
			Status:            notificationStatusPending,
			RetryCount:        2,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
		{
			ID:                209,
			BizID:             1209,
			Key:               "batch_update_key_9",
			Receiver:          "user9@example.com",
			Channel:           notificationChannelEmail,
			TemplateID:        209,
			TemplateVersionID: 2009,
			Status:            notificationStatusPending,
			RetryCount:        3,
			ScheduledSTime:    now,
			ScheduledETime:    now + 3600,
			Ctime:             now,
			Utime:             now,
		},
	}

	err := s.db.CreateInBatches(notifications, len(notifications)).Error
	assert.NoError(t, err)

	// 场景1: 仅更新成功状态
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, []uint64{201, 202, 203}, nil)
	assert.NoError(t, err)

	// 验证更新结果 - 检查每个成功状态的记录
	for _, id := range []uint64{201, 202, 203} {
		var result Notification
		// 使用新的GORM会话避免条件累加
		err = s.db.Session(&gorm.Session{}).Where("id = ?", id).First(&result).Error
		assert.NoError(t, err)

		// 查找原始记录进行比较
		var original Notification
		for _, n := range notifications {
			if n.ID == id {
				original = n
				break
			}
		}

		// 验证状态已更新，其他字段保持不变
		assert.Equal(t, notificationStatusSucceeded, result.Status)
		assert.Equal(t, original.RetryCount, result.RetryCount)
		assert.Equal(t, original.BizID, result.BizID)
		assert.Equal(t, original.Receiver, result.Receiver)
		assert.Equal(t, original.Channel, result.Channel)
		assert.Equal(t, original.TemplateID, result.TemplateID)
		assert.Equal(t, original.ScheduledSTime, result.ScheduledSTime)
		assert.Equal(t, original.ScheduledETime, result.ScheduledETime)
		assert.Greater(t, result.Utime, original.Utime)
	}

	// 场景2: 仅更新失败状态但不更新重试次数
	failedItems1 := []Notification{
		{
			ID:         204,
			RetryCount: 0, // 不更新重试次数
		},
	}
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, nil, failedItems1)
	assert.NoError(t, err)

	// 验证更新结果 - 检查失败状态的记录
	var result Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 204).First(&result).Error
	assert.NoError(t, err)

	// 查找原始记录进行比较
	var original Notification
	for _, n := range notifications {
		if n.ID == 204 {
			original = n
			break
		}
	}

	// 验证状态已更新，重试次数保持不变，其他字段保持不变
	assert.Equal(t, notificationStatusFailed, result.Status)
	assert.Equal(t, original.RetryCount, result.RetryCount)
	assert.Equal(t, original.BizID, result.BizID)
	assert.Equal(t, original.Receiver, result.Receiver)
	assert.Equal(t, original.Channel, result.Channel)
	assert.Equal(t, original.TemplateID, result.TemplateID)
	assert.Equal(t, original.ScheduledSTime, result.ScheduledSTime)
	assert.Equal(t, original.ScheduledETime, result.ScheduledETime)
	assert.Greater(t, result.Utime, original.Utime)

	// 场景3: 仅更新失败状态并更新重试次数
	failedItems2 := []Notification{
		{
			ID:         205,
			RetryCount: 2, // 设置新的重试次数
		},
	}
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, nil, failedItems2)
	assert.NoError(t, err)

	// 验证更新结果 - 检查失败状态和重试次数的记录
	// 使用新的变量和新的会话
	var result2 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 205).First(&result2).Error
	assert.NoError(t, err)

	// 查找原始记录进行比较
	for _, n := range notifications {
		if n.ID == 205 {
			original = n
			break
		}
	}

	// 验证状态和重试次数已更新，其他字段保持不变
	assert.Equal(t, notificationStatusFailed, result2.Status)
	assert.Equal(t, int8(2), result2.RetryCount) // 更新为新的重试次数
	assert.Equal(t, original.BizID, result2.BizID)
	assert.Equal(t, original.Receiver, result2.Receiver)
	assert.Equal(t, original.Channel, result2.Channel)
	assert.Equal(t, original.TemplateID, result2.TemplateID)
	assert.Equal(t, original.ScheduledSTime, result2.ScheduledSTime)
	assert.Equal(t, original.ScheduledETime, result2.ScheduledETime)
	assert.Greater(t, result2.Utime, original.Utime)

	// 场景4: 更新成功状态和失败状态的组合
	successIDs := []uint64{206}
	failedItems3 := []Notification{
		{
			ID:         207,
			RetryCount: 0, // 不更新重试次数
		},
		{
			ID:         208,
			RetryCount: 3, // 更新重试次数
		},
	}
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, successIDs, failedItems3)
	assert.NoError(t, err)

	// 验证成功状态更新
	var result3 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 206).First(&result3).Error
	assert.NoError(t, err)
	assert.Equal(t, notificationStatusSucceeded, result3.Status)

	// 查找原始记录进行比较
	for _, n := range notifications {
		if n.ID == 206 {
			original = n
			break
		}
	}
	assert.Equal(t, original.RetryCount, result3.RetryCount)
	assert.Equal(t, original.BizID, result3.BizID)
	assert.Equal(t, original.Receiver, result3.Receiver)
	assert.Equal(t, original.Channel, result3.Channel)

	// 验证失败状态不更新重试次数
	var result4 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 207).First(&result4).Error
	assert.NoError(t, err)
	assert.Equal(t, notificationStatusFailed, result4.Status)

	// 查找原始记录进行比较
	for _, n := range notifications {
		if n.ID == 207 {
			original = n
			break
		}
	}
	assert.Equal(t, original.RetryCount, result4.RetryCount) // 保持原始重试次数不变
	assert.Equal(t, original.BizID, result4.BizID)
	assert.Equal(t, original.Receiver, result4.Receiver)
	assert.Equal(t, original.Channel, result4.Channel)

	// 验证失败状态更新重试次数
	var result5 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 208).First(&result5).Error
	assert.NoError(t, err)
	assert.Equal(t, notificationStatusFailed, result5.Status)

	// 查找原始记录进行比较
	for _, n := range notifications {
		if n.ID == 208 {
			original = n
			break
		}
	}
	assert.Equal(t, int8(3), result5.RetryCount) // 更新为新的重试次数
	assert.Equal(t, original.BizID, result5.BizID)
	assert.Equal(t, original.Receiver, result5.Receiver)
	assert.Equal(t, original.Channel, result5.Channel)

	// 场景5: 空的参数列表
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, []uint64{}, []Notification{})
	assert.NoError(t, err)
}
