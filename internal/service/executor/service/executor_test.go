package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/sonyflake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	notificationmocks "gitee.com/flycash/notification-platform/internal/service/notification/mocks"
)

func TestExecutorSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutorTestSuite))
}

type ExecutorTestSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockNotifSvc *notificationmocks.MockNotificationService
	idGenerator  *sonyflake.Sonyflake
	executorSvc  ExecutorService
}

func (s *ExecutorTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockNotifSvc = notificationmocks.NewMockNotificationService(s.ctrl)

	// 使用固定设置的ID生成器
	s.idGenerator = sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		MachineID: func() (uint16, error) {
			return 1, nil
		},
	})

	s.executorSvc = NewExecutorService(s.mockNotifSvc, s.idGenerator)
}

func (s *ExecutorTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *ExecutorTestSuite) TestSendNotification_Success() {
	s.T().Skip()

	// 设置mock期望
	s.mockNotifSvc.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n domain.Notification) (domain.Notification, error) {
			n.Status = domain.StatusPending
			return n, nil
		})

	s.mockNotifSvc.EXPECT().
		UpdateNotificationStatus(gomock.Any(), gomock.Any(), gomock.Any(), domain.StatusSucceeded).
		Return(nil)

	s.mockNotifSvc.EXPECT().
		GetNotificationByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint64) (domain.Notification, error) {
			return domain.Notification{
				ID:       id,
				Status:   domain.StatusSucceeded,
				SendTime: time.Now().Unix(),
			}, nil
		})

	// 测试
	notification := Notification{
		BizID:          123,
		Key:            "test-key-1",
		Receiver:       "test@example.com",
		Channel:        ChannelEmail,
		TemplateID:     1001,
		TemplateParams: map[string]string{"name": "Test User"},
		Strategy:       SendStrategyImmediate,
	}

	resp, err := s.executorSvc.SendNotification(context.Background(), notification)

	// 断言
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), resp.NotificationID)
	assert.Equal(s.T(), SendStatusSucceeded, resp.Status)
	assert.NotZero(s.T(), resp.SendTime)
}

func (s *ExecutorTestSuite) TestSendNotification_WithDifferentStrategies() {
	s.T().Skip()

	testCases := []struct {
		name         string
		notification Notification
	}{
		{
			name: "立即发送策略",
			notification: Notification{
				BizID:      123,
				Key:        "test-immediate",
				Receiver:   "user@example.com",
				Channel:    ChannelEmail,
				TemplateID: 1001,
				Strategy:   SendStrategyImmediate,
			},
		},
		{
			name: "延迟发送策略",
			notification: Notification{
				BizID:        123,
				Key:          "test-delayed",
				Receiver:     "user@example.com",
				Channel:      ChannelEmail,
				TemplateID:   1001,
				Strategy:     SendStrategyDelayed,
				DelaySeconds: 30,
			},
		},
		{
			name: "定时发送策略",
			notification: Notification{
				BizID:         123,
				Key:           "test-scheduled",
				Receiver:      "user@example.com",
				Channel:       ChannelEmail,
				TemplateID:    1001,
				Strategy:      SendStrategyScheduled,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
		},
		{
			name: "时间窗口策略",
			notification: Notification{
				BizID:                 123,
				Key:                   "test-window",
				Receiver:              "user@example.com",
				Channel:               ChannelEmail,
				TemplateID:            1001,
				Strategy:              SendStrategyTimeWindow,
				StartTimeMilliseconds: time.Now().UnixMilli(),
				EndTimeMilliseconds:   time.Now().Add(2 * time.Hour).UnixMilli(),
			},
		},
	}

	for i := range testCases {
		s.T().Run(testCases[i].name, func(t *testing.T) {
			// 为每个子测试设置新的mock期望
			s.mockNotifSvc.EXPECT().
				CreateNotification(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, n domain.Notification) (domain.Notification, error) {
					n.Status = domain.StatusPending
					return n, nil
				})

			s.mockNotifSvc.EXPECT().
				UpdateNotificationStatus(gomock.Any(), gomock.Any(), gomock.Any(), domain.StatusSucceeded).
				Return(nil)

			s.mockNotifSvc.EXPECT().
				GetNotificationByID(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, id uint64) (domain.Notification, error) {
					return domain.Notification{
						ID:       id,
						Status:   domain.StatusSucceeded,
						SendTime: time.Now().Unix(),
					}, nil
				})

			resp, err := s.executorSvc.SendNotification(t.Context(), testCases[i].notification)
			require.NoError(t, err)
			assert.NotZero(t, resp.NotificationID)
			assert.Equal(t, SendStatusSucceeded, resp.Status)
		})
	}
}

func (s *ExecutorTestSuite) TestSendNotification_InvalidParameters() {
	testCases := []struct {
		name         string
		notification Notification
		expectedErr  string
	}{
		{
			name:         "业务ID为空",
			notification: Notification{Key: "key", Receiver: "user", Channel: ChannelSMS, TemplateID: 1, Strategy: SendStrategyImmediate},
			expectedErr:  "业务ID不能为空",
		},
		{
			name:         "Key为空",
			notification: Notification{BizID: 123, Receiver: "user", Channel: ChannelSMS, TemplateID: 1, Strategy: SendStrategyImmediate},
			expectedErr:  "业务唯一标识不能为空",
		},
		{
			name:         "接收者为空",
			notification: Notification{BizID: 123, Key: "key", Channel: ChannelSMS, TemplateID: 1, Strategy: SendStrategyImmediate},
			expectedErr:  "接收者不能为空",
		},
		{
			name:         "渠道无效",
			notification: Notification{BizID: 123, Key: "key", Receiver: "user", TemplateID: 1, Strategy: SendStrategyImmediate},
			expectedErr:  "不支持的通知渠道",
		},
		{
			name:         "模板ID无效",
			notification: Notification{BizID: 123, Key: "key", Receiver: "user", Channel: ChannelSMS, Strategy: SendStrategyImmediate},
			expectedErr:  "无效的模板ID",
		},
		{
			name: "延迟策略参数无效",
			notification: Notification{
				BizID:        123,
				Key:          "key",
				Receiver:     "user",
				Channel:      ChannelSMS,
				TemplateID:   1,
				Strategy:     SendStrategyDelayed,
				DelaySeconds: 0, // 无效的延迟秒数
			},
			expectedErr: "延迟发送策略需要指定正数的延迟秒数",
		},
		{
			name: "定时策略参数无效",
			notification: Notification{
				BizID:      123,
				Key:        "key",
				Receiver:   "user",
				Channel:    ChannelSMS,
				TemplateID: 1,
				Strategy:   SendStrategyScheduled,
				// 未指定ScheduledTime
			},
			expectedErr: "定时发送策略需要指定未来的发送时间",
		},
		{
			name: "时间窗口策略参数无效",
			notification: Notification{
				BizID:      123,
				Key:        "key",
				Receiver:   "user",
				Channel:    ChannelSMS,
				TemplateID: 1,
				Strategy:   SendStrategyTimeWindow,
				// 未指定时间窗口参数
			},
			expectedErr: "时间窗口策略需要指定有效的开始和结束时间",
		},
	}

	for i := range testCases {
		s.T().Run(testCases[i].name, func(t *testing.T) {
			// 参数验证失败不会调用mock方法，所以不需要设置期望
			resp, err := s.executorSvc.SendNotification(t.Context(), testCases[i].notification)
			require.NoError(t, err) // 参数错误应该在响应中，而不是错误
			assert.Equal(t, SendStatusFailed, resp.Status)
			assert.Equal(t, ErrorCodeInvalidParameter, resp.ErrorCode)
			assert.Contains(t, resp.ErrorMessage, testCases[i].expectedErr)
		})
	}
}

func (s *ExecutorTestSuite) TestSendNotification_ServiceError() {
	s.T().Skip()

	// 设置mock期望返回错误
	expectedErr := errors.New("service error")

	s.mockNotifSvc.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		Return(domain.Notification{}, expectedErr)

	// 测试
	notification := Notification{
		BizID:      123,
		Key:        "test-key-error",
		Receiver:   "test@example.com",
		Channel:    ChannelEmail,
		TemplateID: 1001,
		Strategy:   SendStrategyImmediate,
	}

	resp, err := s.executorSvc.SendNotification(context.Background(), notification)

	// 断言
	require.NoError(s.T(), err) // 服务错误应该包含在响应中
	assert.Equal(s.T(), SendStatusFailed, resp.Status)
	assert.Equal(s.T(), ErrorCodeUnspecified, resp.ErrorCode)
	assert.Contains(s.T(), resp.ErrorMessage, "service error")
}

func (s *ExecutorTestSuite) TestSendNotificationAsync_Success() {
	// 设置mock期望
	s.mockNotifSvc.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n domain.Notification) (domain.Notification, error) {
			n.Status = domain.StatusPending
			return n, nil
		})

	// 测试
	notification := Notification{
		BizID:      123,
		Key:        "test-key-async",
		Receiver:   "test@example.com",
		Channel:    ChannelEmail,
		TemplateID: 1001,
		Strategy:   SendStrategyImmediate,
	}

	resp, err := s.executorSvc.SendNotificationAsync(context.Background(), notification)

	// 断言
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), resp.NotificationID)
	assert.Equal(s.T(), SendStatusPending, resp.Status)
}

func (s *ExecutorTestSuite) TestBatchSendNotifications_Success() {
	s.T().Skip()

	// 这里我们改用SendNotification而不是BatchSendNotifications，因为executor实现中使用的是SendNotification
	s.mockNotifSvc.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n domain.Notification) (domain.Notification, error) {
			n.Status = domain.StatusPending
			return n, nil
		}).Times(2)

	s.mockNotifSvc.EXPECT().
		UpdateNotificationStatus(gomock.Any(), gomock.Any(), gomock.Any(), domain.StatusSucceeded).
		Return(nil).Times(2)

	s.mockNotifSvc.EXPECT().
		GetNotificationByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint64) (domain.Notification, error) {
			return domain.Notification{
				ID:       id,
				Status:   domain.StatusSucceeded,
				SendTime: time.Now().Unix(),
			}, nil
		}).Times(2)

	// 准备两个通知
	notifications := []Notification{
		{
			BizID:      123,
			Key:        "batch-key-1",
			Receiver:   "user1@example.com",
			Channel:    ChannelEmail,
			TemplateID: 1001,
			Strategy:   SendStrategyImmediate,
		},
		{
			BizID:      124,
			Key:        "batch-key-2",
			Receiver:   "user2@example.com",
			Channel:    ChannelSMS,
			TemplateID: 1002,
			Strategy:   SendStrategyImmediate,
		},
	}

	// 测试
	resp, err := s.executorSvc.BatchSendNotifications(context.Background(), notifications...)

	// 断言
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, resp.TotalCount)
	assert.Equal(s.T(), 2, resp.SuccessCount)
	assert.Len(s.T(), resp.Results, 2)
	assert.Equal(s.T(), SendStatusSucceeded, resp.Results[0].Status)
	assert.Equal(s.T(), SendStatusSucceeded, resp.Results[1].Status)
}

func (s *ExecutorTestSuite) TestBatchSendNotificationsAsync_Success() {
	// 设置mock期望
	s.mockNotifSvc.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n domain.Notification) (domain.Notification, error) {
			// 确保ID被返回
			return n, nil
		}).Times(2)

	// 准备两个通知
	notifications := []Notification{
		{
			BizID:      123,
			Key:        "batch-async-1",
			Receiver:   "user1@example.com",
			Channel:    ChannelEmail,
			TemplateID: 1001,
			Strategy:   SendStrategyImmediate,
		},
		{
			BizID:      124,
			Key:        "batch-async-2",
			Receiver:   "user2@example.com",
			Channel:    ChannelSMS,
			TemplateID: 1002,
			Strategy:   SendStrategyImmediate,
		},
	}

	// 测试
	resp, err := s.executorSvc.BatchSendNotificationsAsync(context.Background(), notifications...)

	// 断言
	require.NoError(s.T(), err)
	assert.Len(s.T(), resp.NotificationIDs, 2)
	assert.NotZero(s.T(), resp.NotificationIDs[0])
	assert.NotZero(s.T(), resp.NotificationIDs[1])
}

func (s *ExecutorTestSuite) TestBatchQueryNotifications_Basic() {
	s.T().Skip()
	// BatchQueryNotifications目前的实现没有调用NotificationService，
	// 但是我们可以在这里添加未来可能的期望

	// 测试
	keys := []string{"query-key-1", "query-key-2"}
	results, err := s.executorSvc.BatchQueryNotifications(context.Background(), keys...)

	// 断言
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 2)
}
