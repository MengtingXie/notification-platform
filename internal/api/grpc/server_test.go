//go:build e2e

package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	executormocks "gitee.com/flycash/notification-platform/internal/service/executor/mocks"
	executorsvc "gitee.com/flycash/notification-platform/internal/service/executor/service"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	notificationSvc notificationsvc.Service
	mockExecutor    *executormocks.MockExecutorService
	ctrl            *gomock.Controller
}

func (s *ServerTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	s.notificationSvc = notificationsvc.InitModule(s.db, testioc.InitIDGenerator()).Svc
	s.listener = bufconn.Listen(1024 * 1024)

	// 创建mock控制器
	s.ctrl = gomock.NewController(s.T())
	// 创建mock执行器
	s.mockExecutor = executormocks.NewMockExecutorService(s.ctrl)

	// 启动grpc.Server
	s.grpcServer = grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(s.grpcServer, grpcapi.NewServer(s.mockExecutor))

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
	s.ctrl.Finish()
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
		setup   func(t *testing.T, req *notificationv1.SendNotificationRequest) // 设置mock期望
	}{
		{
			name: "SMS_立即发送_成功",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-sms-%d", timestamp),
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
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				notificationID := uint64(1000 + timestamp%100)
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						NotificationID: notificationID,
						Status:         executorsvc.SendStatusSucceeded,
						SendTime:       time.Now(),
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)
			},
			wantErr: nil,
		},
		{
			name: "EMAIL_立即发送_成功",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-email-%d", timestamp),
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
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				notificationID := uint64(2000 + timestamp%100)
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						NotificationID: notificationID,
						Status:         executorsvc.SendStatusSucceeded,
						SendTime:       time.Now(),
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)
			},
			wantErr: nil,
		},
		{
			name: "延迟策略_成功",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-delayed-%d", timestamp),
					Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000+1),
					Channel:    notificationv1.Channel_SMS,
					TemplateId: "100",
					TemplateParams: map[string]string{
						"code": "234567",
					},
					Strategy: &notificationv1.SendStrategy{
						StrategyType: &notificationv1.SendStrategy_Delayed{
							Delayed: &notificationv1.SendStrategy_DelayedStrategy{
								DelaySeconds: 60,
							},
						},
					},
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				notificationID := uint64(3000 + timestamp%100)
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						NotificationID: notificationID,
						Status:         executorsvc.SendStatusSucceeded,
						SendTime:       time.Now(),
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)
			},
			wantErr: nil,
		},
		{
			name: "定时策略_成功",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-scheduled-%d", timestamp),
					Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000+2),
					Channel:    notificationv1.Channel_IN_APP,
					TemplateId: "100",
					TemplateParams: map[string]string{
						"code": "789012",
					},
					Strategy: &notificationv1.SendStrategy{
						StrategyType: &notificationv1.SendStrategy_Scheduled{
							Scheduled: &notificationv1.SendStrategy_ScheduledStrategy{
								SendTime: timestamppb.New(time.Now().Add(1 * time.Hour)),
							},
						},
					},
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				notificationID := uint64(4000 + timestamp%100)
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						NotificationID: notificationID,
						Status:         executorsvc.SendStatusSucceeded,
						SendTime:       time.Now(),
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)
			},
			wantErr: nil,
		},
		{
			name: "时间窗口策略_成功",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-timewindow-%d", timestamp),
					Receiver:   fmt.Sprintf("138%010d", timestamp%10000000000+3),
					Channel:    notificationv1.Channel_IN_APP,
					TemplateId: "100",
					TemplateParams: map[string]string{
						"code": "345678",
					},
					Strategy: &notificationv1.SendStrategy{
						StrategyType: &notificationv1.SendStrategy_TimeWindow{
							TimeWindow: &notificationv1.SendStrategy_TimeWindowStrategy{
								StartTimeMilliseconds: time.Now().UnixMilli(),
								EndTimeMilliseconds:   time.Now().Add(3 * time.Hour).UnixMilli(),
							},
						},
					},
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				notificationID := uint64(5000 + timestamp%100)
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						NotificationID: notificationID,
						Status:         executorsvc.SendStatusSucceeded,
						SendTime:       time.Now(),
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
				require.NotNil(t, resp.SendTime)
			},
			wantErr: nil,
		},
		{
			name: "无效的渠道_失败",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-invalid-channel-%d", timestamp),
					Receiver:   fmt.Sprintf("user-invalid-channel-%d", timestamp),
					Channel:    notificationv1.Channel_CHANNEL_UNSPECIFIED,
					TemplateId: "300",
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						Status:       executorsvc.SendStatusFailed,
						ErrorCode:    executorsvc.ErrorCodeInvalidParameter,
						ErrorMessage: "无效的请求参数: 不支持的通知渠道",
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "无效的请求参数")
			},
			wantErr: nil,
		},
		{
			name: "无效的模板ID_失败",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-invalid-template-%d", timestamp),
					Receiver:   fmt.Sprintf("user-invalid-template-%d", timestamp),
					Channel:    notificationv1.Channel_SMS,
					TemplateId: "798",
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						Status:       executorsvc.SendStatusFailed,
						ErrorCode:    executorsvc.ErrorCodeInvalidParameter,
						ErrorMessage: "无效的请求参数: 无效的模板ID",
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "无效的请求参数")
			},
			wantErr: nil,
		},
		{
			name: "接收者为空_失败",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        fmt.Sprintf("test-key-empty-receiver-%d", timestamp),
					Receiver:   "", // 空接收者
					Channel:    notificationv1.Channel_SMS,
					TemplateId: "100",
				},
			},
			setup: func(t *testing.T, req *notificationv1.SendNotificationRequest) {
				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					Return(executorsvc.SendResponse{
						Status:       executorsvc.SendStatusFailed,
						ErrorCode:    executorsvc.ErrorCodeInvalidParameter,
						ErrorMessage: "无效的请求参数: 接收者不能为空",
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.Equal(t, notificationv1.SendStatus_FAILED, resp.Status)
				require.Equal(t, notificationv1.ErrorCode_INVALID_PARAMETER, resp.ErrorCode)
				require.Contains(t, resp.ErrorMessage, "无效的请求参数")
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := s.newClientConn()
			defer conn.Close()

			client := notificationv1.NewNotificationServiceClient(conn)

			// 设置mock期望
			if tc.setup != nil {
				tc.setup(t, tc.req)
			}

			// 创建带有认证信息的上下文
			ctx := metadata.NewOutgoingContext(
				context.Background(),
				metadata.New(map[string]string{
					"Authorization": "Bearer test-token", // 测试用认证Token
				}),
			)

			resp, err := client.SendNotification(ctx, tc.req)

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
