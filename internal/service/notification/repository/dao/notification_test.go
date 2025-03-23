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
		ID:             1,
		BizID:          "test_biz_id_1",
		Receiver:       "user@example.com",
		Channel:        NotificationChannelEmail,
		TemplateID:     101,
		Content:        "测试内容",
		Status:         NotificationStatusPending,
		RetryCount:     0,
		ScheduledSTime: time.Now().Unix(),
		ScheduledETime: time.Now().Add(time.Hour).Unix(),
	}

	err := s.dao.Create(ctx, notification)
	assert.NoError(t, err)

	// 验证创建成功
	var result Notification
	err = s.db.First(&result, "id = ?", notification.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Receiver, result.Receiver)
	assert.Equal(t, notification.Channel, result.Channel)
	assert.Equal(t, notification.Status, result.Status)
	assert.NotZero(t, result.Ctime)
	assert.NotZero(t, result.Utime)
}

func (s *NotificationDAOTestSuite) TestBatchCreate() {
	t := s.T()
	ctx := context.Background()

	notifications := []Notification{
		{
			ID:             2,
			BizID:          "test_biz_id_2",
			Receiver:       "user1@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     102,
			Content:        "用户1测试内容",
			Status:         NotificationStatusPending,
			ScheduledSTime: time.Now().Unix(),
			ScheduledETime: time.Now().Add(time.Hour).Unix(),
		},
		{
			ID:             3,
			BizID:          "test_biz_id_3",
			Receiver:       "user2@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     102,
			Content:        "用户2测试内容",
			Status:         NotificationStatusPending,
			ScheduledSTime: time.Now().Unix(),
			ScheduledETime: time.Now().Add(time.Hour).Unix(),
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
		assert.Equal(t, expected.Receiver, actual.Receiver)
		assert.Equal(t, expected.Channel, actual.Channel)
		assert.Equal(t, expected.TemplateID, actual.TemplateID)
		assert.Equal(t, expected.Content, actual.Content)
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
		ID:             4,
		BizID:          "test_biz_id_4",
		Receiver:       "user@example.com",
		Channel:        NotificationChannelEmail,
		TemplateID:     104,
		Content:        "状态更新测试",
		Status:         NotificationStatusPending,
		ScheduledSTime: now.Unix(),
		ScheduledETime: now.Add(time.Hour).Unix(),
		Ctime:          now.Unix(),
		Utime:          now.Unix(),
	}

	err := s.db.Create(&notification).Error
	assert.NoError(t, err)

	// 测试更新状态
	err = s.dao.UpdateStatus(ctx, notification.ID, notification.BizID, NotificationStatusSucceeded)
	assert.NoError(t, err)

	// 验证状态已更新
	var result Notification
	err = s.db.First(&result, notification.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, NotificationStatusSucceeded, result.Status)
	assert.Greater(t, result.Utime, notification.Utime)
}

func (s *NotificationDAOTestSuite) TestFindByID() {
	t := s.T()
	ctx := context.Background()

	// 准备测试数据
	notification := Notification{
		ID:             5,
		BizID:          "test_biz_id_5",
		Receiver:       "user@example.com",
		Channel:        NotificationChannelEmail,
		TemplateID:     105,
		Content:        "ID查询测试",
		Status:         NotificationStatusPending,
		ScheduledSTime: time.Now().Unix(),
		ScheduledETime: time.Now().Add(time.Hour).Unix(),
		Ctime:          time.Now().Unix(),
		Utime:          time.Now().Unix(),
	}

	err := s.db.Create(&notification).Error
	assert.NoError(t, err)

	// 测试查询
	result, err := s.dao.FindByID(ctx, notification.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, Notification{}, result) // 确保不是空结构体
	assert.Equal(t, notification.ID, result.ID)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Content, result.Content)

	assert.Equal(t, notification.ID, result.ID)
	assert.Equal(t, notification.BizID, result.BizID)
	assert.Equal(t, notification.Receiver, result.Receiver)
	assert.Equal(t, notification.Channel, result.Channel)
	assert.Equal(t, notification.TemplateID, result.TemplateID)
	assert.Equal(t, notification.Content, result.Content)
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
	bizID := "test_biz_id_6"
	notifications := []Notification{
		{
			ID:             6,
			BizID:          bizID,
			Receiver:       "user1@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     106,
			Content:        "BizID查询测试1",
			Status:         NotificationStatusPending,
			ScheduledSTime: time.Now().Unix(),
			ScheduledETime: time.Now().Add(time.Hour).Unix(),
			Ctime:          time.Now().Unix(),
			Utime:          time.Now().Unix(),
		},
		{
			ID:             7,
			BizID:          bizID,
			Receiver:       "user2@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     106,
			Content:        "BizID查询测试2",
			Status:         NotificationStatusPending,
			ScheduledSTime: time.Now().Unix(),
			ScheduledETime: time.Now().Add(time.Hour).Unix(),
			Ctime:          time.Now().Unix(),
			Utime:          time.Now().Unix(),
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
		assert.Equal(t, expected.Receiver, result.Receiver)
		assert.Equal(t, expected.Channel, result.Channel)
		assert.Equal(t, expected.TemplateID, result.TemplateID)
		assert.Equal(t, expected.Content, result.Content)
		assert.Equal(t, expected.Status, result.Status)
		assert.Equal(t, expected.ScheduledSTime, result.ScheduledSTime)
		assert.Equal(t, expected.ScheduledETime, result.ScheduledETime)
		assert.Equal(t, expected.Ctime, result.Ctime)
		assert.Equal(t, expected.Utime, result.Utime)
	}

	// 测试查询不存在记录
	results, err = s.dao.FindByBizID(ctx, "not_exist_biz_id")
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
			ID:             9,
			BizID:          "test_biz_id_9",
			Receiver:       "user1@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     109,
			Content:        "状态查询测试1",
			Status:         NotificationStatusPending,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             10,
			BizID:          "test_biz_id_10",
			Receiver:       "user2@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     110,
			Content:        "状态查询测试2",
			Status:         NotificationStatusSucceeded,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             11,
			BizID:          "test_biz_id_11",
			Receiver:       "user3@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     111,
			Content:        "状态查询测试3",
			Status:         NotificationStatusPending,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
	}

	err := s.db.CreateInBatches(notifications, len(notifications)).Error
	assert.NoError(t, err)

	// 测试查询 - 验证PENDING状态的记录
	results, err := s.dao.ListByStatus(ctx, NotificationStatusPending, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试查询 - 验证SUCCEEDED状态的记录
	results, err = s.dao.ListByStatus(ctx, NotificationStatusSucceeded, 10)
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
			ID:             12,
			BizID:          "test_biz_id_12",
			Receiver:       "user1@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     112,
			Content:        "时间查询测试1",
			Status:         NotificationStatusPending,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             13,
			BizID:          "test_biz_id_13",
			Receiver:       "user2@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     113,
			Content:        "时间查询测试2",
			Status:         NotificationStatusPending,
			ScheduledSTime: laterTime,
			ScheduledETime: laterTime + 3600,
			Ctime:          now,
			Utime:          now,
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
	assert.Equal(t, notifications[0].Receiver, results[0].Receiver)
	assert.Equal(t, notifications[0].Channel, results[0].Channel)
	assert.Equal(t, notifications[0].TemplateID, results[0].TemplateID)
	assert.Equal(t, notifications[0].Content, results[0].Content)
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
		assert.Equal(t, expected.Receiver, result.Receiver)
		assert.Equal(t, expected.Channel, result.Channel)
		assert.Equal(t, expected.TemplateID, result.TemplateID)
		assert.Equal(t, expected.Content, result.Content)
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
			ID:             201,
			BizID:          "batch_update_1",
			Receiver:       "user1@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     201,
			Content:        "批量更新测试1",
			Status:         NotificationStatusPending,
			RetryCount:     0,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             202,
			BizID:          "batch_update_2",
			Receiver:       "user2@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     202,
			Content:        "批量更新测试2",
			Status:         NotificationStatusPending,
			RetryCount:     0,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             203,
			BizID:          "batch_update_3",
			Receiver:       "user3@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     203,
			Content:        "批量更新测试3",
			Status:         NotificationStatusPending,
			RetryCount:     0,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             204,
			BizID:          "batch_update_4",
			Receiver:       "user4@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     204,
			Content:        "批量更新测试4",
			Status:         NotificationStatusPending,
			RetryCount:     1,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             205,
			BizID:          "batch_update_5",
			Receiver:       "user5@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     205,
			Content:        "批量更新测试5",
			Status:         NotificationStatusPending,
			RetryCount:     1,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             206,
			BizID:          "batch_update_6",
			Receiver:       "user6@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     206,
			Content:        "批量更新测试6",
			Status:         NotificationStatusPending,
			RetryCount:     1,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             207,
			BizID:          "batch_update_7",
			Receiver:       "user7@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     207,
			Content:        "批量更新测试7",
			Status:         NotificationStatusPending,
			RetryCount:     2,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             208,
			BizID:          "batch_update_8",
			Receiver:       "user8@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     208,
			Content:        "批量更新测试8",
			Status:         NotificationStatusPending,
			RetryCount:     2,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
		},
		{
			ID:             209,
			BizID:          "batch_update_9",
			Receiver:       "user9@example.com",
			Channel:        NotificationChannelEmail,
			TemplateID:     209,
			Content:        "批量更新测试9",
			Status:         NotificationStatusPending,
			RetryCount:     3,
			ScheduledSTime: now,
			ScheduledETime: now + 3600,
			Ctime:          now,
			Utime:          now,
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
		assert.Equal(t, NotificationStatusSucceeded, result.Status)
		assert.Equal(t, original.RetryCount, result.RetryCount)
		assert.Equal(t, original.BizID, result.BizID)
		assert.Equal(t, original.Receiver, result.Receiver)
		assert.Equal(t, original.Channel, result.Channel)
		assert.Equal(t, original.TemplateID, result.TemplateID)
		assert.Equal(t, original.Content, result.Content)
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
	assert.Equal(t, NotificationStatusFailed, result.Status)
	assert.Equal(t, original.RetryCount, result.RetryCount)
	assert.Equal(t, original.BizID, result.BizID)
	assert.Equal(t, original.Receiver, result.Receiver)
	assert.Equal(t, original.Channel, result.Channel)
	assert.Equal(t, original.TemplateID, result.TemplateID)
	assert.Equal(t, original.Content, result.Content)
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
	assert.Equal(t, NotificationStatusFailed, result2.Status)
	assert.Equal(t, int8(2), result2.RetryCount) // 更新为新的重试次数
	assert.Equal(t, original.BizID, result2.BizID)
	assert.Equal(t, original.Receiver, result2.Receiver)
	assert.Equal(t, original.Channel, result2.Channel)
	assert.Equal(t, original.TemplateID, result2.TemplateID)
	assert.Equal(t, original.Content, result2.Content)
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
	assert.Equal(t, NotificationStatusSucceeded, result3.Status)

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
	assert.Equal(t, original.Content, result3.Content)

	// 验证失败状态不更新重试次数
	var result4 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 207).First(&result4).Error
	assert.NoError(t, err)
	assert.Equal(t, NotificationStatusFailed, result4.Status)

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
	assert.Equal(t, original.Content, result4.Content)

	// 验证失败状态更新重试次数
	var result5 Notification
	err = s.db.Session(&gorm.Session{}).Where("id = ?", 208).First(&result5).Error
	assert.NoError(t, err)
	assert.Equal(t, NotificationStatusFailed, result5.Status)

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
	assert.Equal(t, original.Content, result5.Content)

	// 场景5: 空的参数列表
	err = s.dao.BatchUpdateStatusSucceededOrFailed(ctx, []uint64{}, []Notification{})
	assert.NoError(t, err)
}

func TestNotificationDAOSuite(t *testing.T) {
	suite.Run(t, new(NotificationDAOTestSuite))
}
