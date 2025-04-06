//go:build e2e

package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/jwt"
	txnotification "gitee.com/flycash/notification-platform/internal/service/tx_notification"
	txnotificationmocks "gitee.com/flycash/notification-platform/internal/service/tx_notification/mocks"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	grpcapi "gitee.com/flycash/notification-platform/internal/api/grpc"
	executorsvc "gitee.com/flycash/notification-platform/internal/service/executor"
	executormocks "gitee.com/flycash/notification-platform/internal/service/executor/mocks"
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
	jwtAuth         *jwt.JwtAuth
}

func (s *ServerTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	s.notificationSvc = notificationsvc.InitModule(s.db, testioc.InitIDGenerator()).Svc
	s.listener = bufconn.Listen(1024 * 1024)
	s.jwtAuth = jwt.NewJwtAuth("key1")
	// 创建mock控制器
	s.ctrl = gomock.NewController(s.T())
	// 创建mock执行器
	s.mockExecutor = executormocks.NewMockExecutorService(s.ctrl)

	// 启动grpc.Server
	s.grpcServer = grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(s.grpcServer, grpcapi.NewServer(s.mockExecutor, nil))

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

// 定义一个基本结构，之后只需要修改几个地方
type mockExecutorNotification struct {
	Notification       notificationsvc.Notification
	SendStrategyConfig struct {
		Type                  executorsvc.SendStrategyType
		DelaySeconds          int64
		ScheduledTime         time.Time
		StartTimeMilliseconds int64
		EndTimeMilliseconds   int64
	}
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
				// 期望转换后的notification
				expectedNotification := executorsvc.Notification{
					Notification: notificationsvc.Notification{
						BizID:    int64(101), // 测试用bizID
						Key:      req.Notification.Key,
						Receiver: req.Notification.Receiver,
						Channel:  notificationsvc.ChannelSMS,
						Template: notificationsvc.Template{
							ID:     100,
							Params: req.Notification.TemplateParams,
						},
					},
					SendStrategyConfig: executorsvc.SendStrategyConfig{
						Type: executorsvc.SendStrategyImmediate,
					},
				}

				s.mockExecutor.EXPECT().
					SendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, n executorsvc.Notification) (executorsvc.SendResponse, error) {
						// 确认参数正确
						require.Equal(t, expectedNotification.Notification.BizID, n.Notification.BizID)
						require.Equal(t, expectedNotification.Notification.Key, n.Notification.Key)
						require.Equal(t, expectedNotification.Notification.Channel, n.Notification.Channel)
						require.Equal(t, expectedNotification.SendStrategyConfig.Type, n.SendStrategyConfig.Type)

						// 返回模拟响应
						return executorsvc.SendResponse{
							NotificationID: notificationID,
							Status:         notificationsvc.SendStatusSucceeded,
						}, nil
					})
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
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
						Status:         notificationsvc.SendStatusSucceeded,
					}, nil)
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
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
					DoAndReturn(func(_ context.Context, n executorsvc.Notification) (executorsvc.SendResponse, error) {
						// 验证延迟策略被正确转换
						require.Equal(t, executorsvc.SendStrategyDelayed, n.SendStrategyConfig.Type)
						require.Equal(t, int64(60), n.SendStrategyConfig.DelaySeconds)

						return executorsvc.SendResponse{
							NotificationID: notificationID,
							Status:         notificationsvc.SendStatusSucceeded,
						}, nil
					})
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
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
					DoAndReturn(func(_ context.Context, n executorsvc.Notification) (executorsvc.SendResponse, error) {
						// 验证定时策略被正确转换
						require.Equal(t, executorsvc.SendStrategyScheduled, n.SendStrategyConfig.Type)
						require.False(t, n.SendStrategyConfig.ScheduledTime.IsZero())

						return executorsvc.SendResponse{
							NotificationID: notificationID,
							Status:         notificationsvc.SendStatusSucceeded,
						}, nil
					})
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
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
					DoAndReturn(func(_ context.Context, n executorsvc.Notification) (executorsvc.SendResponse, error) {
						// 验证时间窗口策略被正确转换
						require.Equal(t, executorsvc.SendStrategyTimeWindow, n.SendStrategyConfig.Type)
						require.True(t, n.SendStrategyConfig.StartTimeMilliseconds > 0)
						require.True(t, n.SendStrategyConfig.EndTimeMilliseconds > n.SendStrategyConfig.StartTimeMilliseconds)

						return executorsvc.SendResponse{
							NotificationID: notificationID,
							Status:         notificationsvc.SendStatusSucceeded,
						}, nil
					})
			},
			after: func(t *testing.T, req *notificationv1.SendNotificationRequest, resp *notificationv1.SendNotificationResponse) {
				require.NotEmpty(t, resp.NotificationId)
				require.Equal(t, notificationv1.SendStatus_SUCCEEDED, resp.Status)
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
						Status:       notificationsvc.SendStatusFailed,
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
						Status:       notificationsvc.SendStatusFailed,
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
						Status:       notificationsvc.SendStatusFailed,
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

func (s *ServerTestSuite) TestCommit() {
	mockTxSvc := txnotificationmocks.NewMockTxNotificationService(s.ctrl)
	mockTxSvc.EXPECT().Commit(gomock.Any(), int64(13), "case1").Return(nil)
	// 创建内存监听器
	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	// 创建gRPC服务器
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		jwt.JwtAuthInterceptor(s.jwtAuth),
	))
	defer server.Stop()
	notificationv1.RegisterNotificationServiceServer(server, grpcapi.NewServer(nil, mockTxSvc))
	// 启动服务器
	go func() {
		if err := server.Serve(lis); err != nil {
			s.T().Errorf("gRPC server failed: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 客户端连接配置
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(s.T(), err)
	defer conn.Close()

	// 创建客户端
	client := notificationv1.NewNotificationServiceClient(conn)

	token, err := s.jwtAuth.Encode(map[string]any{
		"biz_id": 13,
	})
	require.NoError(s.T(), err)
	ctx = s.generalToken(ctx, token)
	// 调用Commit方法
	_, verr := client.TxCommit(ctx, &notificationv1.TxCommitRequest{
		Key: "case1",
	})
	require.NoError(s.T(), verr)
}

func (s *ServerTestSuite) TestCancel() {
	mockTxSvc := txnotificationmocks.NewMockTxNotificationService(s.ctrl)
	mockTxSvc.EXPECT().Cancel(gomock.Any(), int64(13), "case1").Return(nil)
	// 创建内存监听器
	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	// 创建gRPC服务器
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		jwt.JwtAuthInterceptor(s.jwtAuth),
	))
	defer server.Stop()
	notificationv1.RegisterNotificationServiceServer(server, grpcapi.NewServer(nil, mockTxSvc))
	// 启动服务器
	go func() {
		if err := server.Serve(lis); err != nil {
			s.T().Errorf("gRPC server failed: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 客户端连接配置
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(s.T(), err)
	defer conn.Close()

	// 创建客户端
	client := notificationv1.NewNotificationServiceClient(conn)

	token, err := s.jwtAuth.Encode(map[string]any{
		"biz_id": 13,
	})
	require.NoError(s.T(), err)
	ctx = s.generalToken(ctx, token)
	// 调用Commit方法
	_, verr := client.TxCancel(ctx, &notificationv1.TxCancelRequest{
		Key: "case1",
	})
	require.NoError(s.T(), verr)
}

func (s *ServerTestSuite) TestPrepare() {
	mockTxSvc := txnotificationmocks.NewMockTxNotificationService(s.ctrl)
	timestamp := time.Now().UnixNano() // Use nanosecond timestamp for uniqueness

	// Create test cases for different strategies
	testCases := []struct {
		name      string
		input     notificationv1.Notification
		setupMock func(t *testing.T, noti notificationv1.Notification)
	}{
		{
			name: "Immediate strategy",
			input: notificationv1.Notification{
				Key:        fmt.Sprintf("test-key-immediate-%d", timestamp),
				Receiver:   "13800138000",
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
			setupMock: func(t *testing.T, noti notificationv1.Notification) {
				mockTxSvc.EXPECT().
					Prepare(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txn txnotification.TxNotification) (uint64, error) {
						// Verify basic properties
						now := time.Now()
						assert.Equal(t, int64(13), txn.BizID)
						assert.Equal(t, noti.Key, txn.Key)
						assert.GreaterOrEqual(t, now.UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(-1*time.Second).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(59*time.Minute).UnixMilli(), txn.Notification.ScheduledETime)
						txn.Notification.ScheduledETime = 0
						txn.Notification.ScheduledSTime = 0
						assert.Equal(t, notificationsvc.Notification{
							BizID:    13,
							Key:      fmt.Sprintf("test-key-immediate-%d", timestamp),
							Receiver: "13800138000",
							Channel:  notificationsvc.ChannelSMS,
							Template: notificationsvc.Template{
								ID: 100,
								Params: map[string]string{
									"code": "123456",
								},
							},
							Status: notificationsvc.SendStatusPrepare,
						}, txn.Notification)
						return 12345, nil
					})
			},
		},
		{
			name: "Delayed strategy",
			input: notificationv1.Notification{
				Key:        fmt.Sprintf("test-key-delayed-%d", timestamp),
				Receiver:   "13800138001",
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
			setupMock: func(t *testing.T, noti notificationv1.Notification) {
				mockTxSvc.EXPECT().
					Prepare(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txn txnotification.TxNotification) (uint64, error) {
						// Verify basic properties
						now := time.Now()
						assert.Equal(t, int64(13), txn.BizID)
						assert.Equal(t, noti.Key, txn.Key)
						assert.GreaterOrEqual(t, now.Add(60*time.Second).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(59*time.Second).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(69*time.Second).UnixMilli(), txn.Notification.ScheduledETime)
						txn.Notification.ScheduledETime = 0
						txn.Notification.ScheduledSTime = 0
						assert.Equal(t, notificationsvc.Notification{
							BizID:    13,
							Key:      fmt.Sprintf("test-key-delayed-%d", timestamp),
							Receiver: "13800138001",
							Channel:  notificationsvc.ChannelSMS,
							Template: notificationsvc.Template{
								ID: 100,
								Params: map[string]string{
									"code": "234567",
								},
							},
							Status: notificationsvc.SendStatusPrepare,
						}, txn.Notification)
						return 12345, nil
					})
			},
		},
		{
			name: "Scheduled strategy",
			input: notificationv1.Notification{
				Key:        fmt.Sprintf("test-key-scheduled-%d", timestamp),
				Receiver:   "13800138002",
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "100",
				TemplateParams: map[string]string{
					"code": "345678",
				},
				Strategy: &notificationv1.SendStrategy{
					StrategyType: &notificationv1.SendStrategy_Scheduled{
						Scheduled: &notificationv1.SendStrategy_ScheduledStrategy{
							SendTime: timestamppb.New(time.Now().Add(1 * time.Hour)),
						},
					},
				},
			},
			setupMock: func(t *testing.T, noti notificationv1.Notification) {
				mockTxSvc.EXPECT().
					Prepare(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txn txnotification.TxNotification) (uint64, error) {
						// Verify basic properties
						now := time.Now()
						assert.Equal(t, int64(13), txn.BizID)
						assert.Equal(t, noti.Key, txn.Key)
						assert.GreaterOrEqual(t, now.Add(60*time.Minute).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(59*time.Minute).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(60*time.Second).UnixMilli(), txn.Notification.ScheduledETime)
						txn.Notification.ScheduledETime = 0
						txn.Notification.ScheduledSTime = 0
						assert.Equal(t, notificationsvc.Notification{
							BizID:    13,
							Key:      fmt.Sprintf("test-key-scheduled-%d", timestamp),
							Receiver: "13800138002",
							Channel:  notificationsvc.ChannelSMS,
							Template: notificationsvc.Template{
								ID: 100,
								Params: map[string]string{
									"code": "345678",
								},
							},
							Status: notificationsvc.SendStatusPrepare,
						}, txn.Notification)
						return 12345, nil
					})
			},
		},
		{
			name: "TimeWindow strategy",
			input: notificationv1.Notification{
				Key:        fmt.Sprintf("test-key-timewindow-%d", timestamp),
				Receiver:   "13800138003",
				Channel:    notificationv1.Channel_SMS,
				TemplateId: "100",
				TemplateParams: map[string]string{
					"code": "456789",
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
			setupMock: func(t *testing.T, noti notificationv1.Notification) {
				mockTxSvc.EXPECT().
					Prepare(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txn txnotification.TxNotification) (uint64, error) {
						// Verify basic properties
						now := time.Now()
						assert.Equal(t, int64(13), txn.BizID)
						assert.Equal(t, noti.Key, txn.Key)
						assert.GreaterOrEqual(t, now.UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(-1*time.Second).UnixMilli(), txn.Notification.ScheduledSTime)
						assert.LessOrEqual(t, now.Add(179*time.Minute).UnixMilli(), txn.Notification.ScheduledETime)
						txn.Notification.ScheduledETime = 0
						txn.Notification.ScheduledSTime = 0
						assert.Equal(t, notificationsvc.Notification{
							BizID:    13,
							Key:      fmt.Sprintf("test-key-timewindow-%d", timestamp),
							Receiver: "13800138003",
							Channel:  notificationsvc.ChannelSMS,
							Template: notificationsvc.Template{
								ID: 100,
								Params: map[string]string{
									"code": "456789",
								},
							},
							Status: notificationsvc.SendStatusPrepare,
						}, txn.Notification)
						return 12345, nil
					})
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// Create memory listener
			lis := bufconn.Listen(1024 * 1024)
			defer lis.Close()

			// Create gRPC server with the mock service
			server := grpc.NewServer(grpc.ChainUnaryInterceptor(
				jwt.JwtAuthInterceptor(s.jwtAuth),
			))
			defer server.Stop()
			notificationv1.RegisterNotificationServiceServer(server, grpcapi.NewServer(nil, mockTxSvc))

			// Start server
			go func() {
				if err := server.Serve(lis); err != nil {
					t.Errorf("gRPC server failed: %v", err)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Client connection setup
			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
					return lis.Dial()
				}),
				grpc.WithInsecure(),
			)
			require.NoError(t, err)
			defer conn.Close()

			// Create client
			client := notificationv1.NewNotificationServiceClient(conn)

			// Setup mock expectations
			tc.setupMock(t, tc.input)

			// Generate token with biz_id
			token, err := s.jwtAuth.Encode(map[string]any{
				"biz_id": 13,
			})
			require.NoError(t, err)
			ctx = s.generalToken(ctx, token)

			// Call TxPrepare method
			response, err := client.TxPrepare(ctx, &notificationv1.TxPrepareRequest{
				Notification: &tc.input,
			})

			// Verify results
			require.NoError(t, err)
			require.NotNil(t, response)
		})
	}
}

func (s *ServerTestSuite) generalToken(ctx context.Context, token string) context.Context {
	md := metadata.Pairs(
		"authorization", token,
	)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx
}
