//go:build e2e

package grpc_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"testing"
	"time"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	notificationioc "gitee.com/flycash/notification-platform/internal/service/notification/ioc"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification/service"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/sony/sonyflake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
)

func TestServer(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

type ServerTestSuite struct {
	suite.Suite

	grpcServer *grpc.Server
	listener   *bufconn.Listener

	db              *gorm.DB
	notificationSvc notificationsvc.NotificationService
}

func (s *ServerTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	s.notificationSvc = notificationioc.InitService(s.db).Svc
	s.listener = bufconn.Listen(1024 * 1024)

	flake := sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Now(), // 或者指定一个固定的开始时间
		MachineID: func() (uint16, error) {
			return 1, nil // 替换为你的机器 ID 获取函数
		},
	})

	// 启动grpc.Server
	s.grpcServer = grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(s.grpcServer, grpcapi.NewServer(s.notificationSvc, flake))

	ready := make(chan struct{})
	go func() {
		close(ready)
		if err := s.grpcServer.Serve(s.listener); err != nil {
			s.NoError(err, "gRPC Server exited")
		}
	}()
	<-ready
}

func (s *ServerTestSuite) TearDownSuite() {
	// 关闭grpc.Server
	s.grpcServer.Stop()
	s.db.Exec("TRUNCATE TABLE notifications")
}

func (s *ServerTestSuite) newClientConn() *grpc.ClientConn {
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return s.listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	s.NoError(err)
	return conn
}

func (s *ServerTestSuite) TestSendNotification() {
	t := s.T()
	timestamp := time.Now().UnixNano() // 使用纳秒级时间戳确保唯一性

	testCases := []struct {
		name    string
		req     *notificationv1.SendNotificationRequest
		after   func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse)
		wantErr error
	}{
		{
			name: "SMS_立即发送_成功",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-sms-%d", timestamp),
				BizId:      fmt.Sprintf("biz-sms-%d", timestamp),
				Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000),
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "100",
				TemplateParams: map[string]string{
					"code": "123456",
				},
				Strategy: &notificationv1.SendStrategy{
					StrategyType: &notificationv1.SendStrategy_Immediate{
						Immediate: &notificationv1.SendStrategy_ImmediateStrategy{},
					},
				},
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, fmt.Sprintf("test-key-sms-%d", timestamp), resp.RequestKey)
				log.Printf("nid = %#v\n", resp.NotificationId)
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)

				// 校验通知记录
				notificationId, err := strconv.ParseInt(resp.NotificationId, 10, 64)
				assert.NoError(t, err)

				s.checkNotification(t, domain.Notification{
					ID:         uint64(notificationId),
					BizID:      req.BizId,
					Receiver:   req.Receiver,
					Channel:    domain.ChannelSMS,
					TemplateID: 100,
					Content:    "xxx",
					Status:     domain.StatusSucceeded,
				})
			},
			wantErr: nil,
		},
		{
			name: "EMAIL_立即发送_成功",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-email-%d", timestamp),
				BizId:      fmt.Sprintf("biz-email-%d", timestamp),
				Receiver:   fmt.Sprintf("test%d@example.com", timestamp),
				Channel:    notificationv1.Channel_EMAIL,
				TemplateId: "200",
				TemplateParams: map[string]string{
					"name":    "张三",
					"content": "邮件内容",
				},
				Strategy: &notificationv1.SendStrategy{
					StrategyType: &notificationv1.SendStrategy_Immediate{
						Immediate: &notificationv1.SendStrategy_ImmediateStrategy{},
					},
				},
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, fmt.Sprintf("test-key-email-%d", timestamp), resp.RequestKey)
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)

				// 校验通知记录
				notificationId, err := strconv.ParseInt(resp.NotificationId, 10, 64)
				assert.NoError(t, err)

				s.checkNotification(t, domain.Notification{
					ID:         uint64(notificationId),
					BizID:      req.BizId,
					Receiver:   req.Receiver,
					Channel:    domain.ChannelEmail,
					TemplateID: 200,
					Content:    "xxx",
					Status:     domain.StatusSucceeded,
				})
			},
			wantErr: nil,
		},
		{
			name: "无效的渠道_失败",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-invalid-channel-%d", timestamp),
				BizId:      fmt.Sprintf("biz-invalid-channel-%d", timestamp),
				Receiver:   fmt.Sprintf("user-invalid-channel-%d", timestamp),
				Channel:    notificationv1.Channel_CHANNEL_UNSPECIFIED,
				TemplateId: "300",
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "不支持的通知渠道")
			},
			wantErr: nil,
		},
		{
			name: "无效的模板ID_失败",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-invalid-template-%d", timestamp),
				BizId:      fmt.Sprintf("biz-invalid-template-%d", timestamp),
				Receiver:   fmt.Sprintf("user-invalid-template-%d", timestamp),
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "invalid",
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "无效的模板ID")
			},
			wantErr: nil,
		},
		{
			name: "业务ID为空_失败",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-empty-bizid-%d", timestamp),
				BizId:      "", // 空业务ID
				Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000+1),
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "100",
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "业务ID不能为空")
			},
			wantErr: nil,
		},
		{
			name: "接收者为空_失败",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-empty-receiver-%d", timestamp),
				BizId:      fmt.Sprintf("biz-empty-receiver-%d", timestamp),
				Receiver:   "", // 空接收者
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "100",
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "接收者不能为空")
			},
			wantErr: nil,
		},
		{
			name: "定时策略发送_成功",
			req: &notificationv1.SendNotificationRequest{
				Key:        fmt.Sprintf("test-key-scheduled-%d", timestamp),
				BizId:      fmt.Sprintf("biz-scheduled-%d", timestamp),
				Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000+2),
				Channel:    notificationv1.Channel_IN_APP,
				TemplateId: "100",
				TemplateParams: map[string]string{
					"code": "789012",
				},
				Strategy: &notificationv1.SendStrategy{
					StrategyType: &notificationv1.SendStrategy_Scheduled{
						Scheduled: &notificationv1.SendStrategy_ScheduledStrategy{
							SendTime: nil, // 使用服务器端当前时间
						},
					},
				},
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, fmt.Sprintf("test-key-scheduled-%d", timestamp), resp.RequestKey)
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)

				// 校验通知记录
				notificationId, err := strconv.ParseInt(resp.NotificationId, 10, 64)
				assert.NoError(t, err)

				s.checkNotification(t, domain.Notification{
					ID:         uint64(notificationId),
					BizID:      req.BizId,
					Receiver:   req.Receiver,
					Channel:    domain.ChannelInApp,
					TemplateID: 100,
					Content:    "xxx",
					Status:     domain.StatusSucceeded,
				})
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := s.newClientConn()
			defer conn.Close()

			client := notificationv1.NewNotificationServiceClient(conn)
			resp, err := client.SendNotification(context.Background(), tc.req)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, tc.wantErr.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			tc.after(t, tc.req, resp)
		})
	}
}

func (s *ServerTestSuite) checkNotification(t *testing.T, expected domain.Notification) {
	actual, err := s.notificationSvc.GetNotificationByID(context.Background(), expected.ID)
	require.NoError(t, err)
	assert.Equal(t, expected.BizID, actual.BizID)
	assert.Equal(t, expected.Receiver, actual.Receiver)
	assert.Equal(t, expected.Channel, actual.Channel)
	assert.Equal(t, expected.TemplateID, actual.TemplateID)
	assert.NotEmpty(t, actual.Content)
	assert.Equal(t, domain.StatusSucceeded, actual.Status)
	assert.Zero(t, actual.RetryCount)
	assert.NotZero(t, actual.ScheduledSTime)
	assert.NotZero(t, actual.ScheduledETime)
	assert.NotZero(t, actual.Ctime)
	assert.NotZero(t, actual.Utime)
}
