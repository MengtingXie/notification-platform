//go:build e2e

package integration

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	"gitee.com/flycash/notification-platform/internal/domain"
	prodioc "gitee.com/flycash/notification-platform/internal/ioc"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
	smsmocks "gitee.com/flycash/notification-platform/internal/service/provider/sms/client/mocks"
	platformioc "gitee.com/flycash/notification-platform/internal/test/integration/ioc/platform"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/google/go-cmp/cmp"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
	"github.com/gotomicro/ego/server/egovernor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/testing/protocmp"
	"gopkg.in/yaml.v2"
)

func TestGRPCServerWithSuccessMock(t *testing.T) {
	suite.Run(t, new(SuccessGRPCServerTestSuite))
}

func TestGRPCServerWithFailureMock(t *testing.T) {
	suite.Run(t, new(FailureGRPCServerTestSuite))
}

// 成功测试套件
type SuccessGRPCServerTestSuite struct {
	BaseGRPCServerTestSuite
}

func (s *SuccessGRPCServerTestSuite) SetupSuite() {
	// 创建成功响应的模拟客户端
	ctrl := gomock.NewController(s.T())
	mockClient := smsmocks.NewMockClient(ctrl)
	mockClient.EXPECT().Send(gomock.Any()).Return(client.SendResp{
		RequestID: "mock-req-id",
		BizID:     "mock-biz-id",
	}, nil).AnyTimes()

	// 配置参数
	clients := map[string]client.Client{"mock-provider": mockClient}
	serverProt := "9004"
	clientAddr := "127.0.0.1:" + serverProt
	s.BaseGRPCServerTestSuite.SetupTestSuite(serverProt, clientAddr, clients)
}

func (s *SuccessGRPCServerTestSuite) TearDownSuite() {
	s.BaseGRPCServerTestSuite.TearDownTestSuite()
}

type FailureGRPCServerTestSuite struct {
	BaseGRPCServerTestSuite
}

func (s *FailureGRPCServerTestSuite) SetupSuite() {
	// 创建失败响应的模拟客户端
	ctrl := gomock.NewController(s.T())
	mockClient := smsmocks.NewMockClient(ctrl)
	mockClient.EXPECT().Send(gomock.Any()).Return(client.SendResp{},
		errors.New("供应商API错误")).AnyTimes()

	// 配置参数，使用不同端口
	clients := map[string]client.Client{"mock-provider": mockClient}
	serverProt := "9005"
	clientAddr := "127.0.0.1:" + serverProt

	s.BaseGRPCServerTestSuite.SetupTestSuite(serverProt, clientAddr, clients)
}

func (s *FailureGRPCServerTestSuite) TearDownSuite() {
	s.BaseGRPCServerTestSuite.TearDownTestSuite()
}

// 基础测试套件
type BaseGRPCServerTestSuite struct {
	suite.Suite
	db     *egorm.Component
	server *ego.Ego
	app    *testioc.App

	client      notificationv1.NotificationServiceClient
	queryClient notificationv1.NotificationQueryServiceClient

	mockClients map[string]client.Client
	serverAddr  string
	clientAddr  string
}

// 设置测试环境
func (s *BaseGRPCServerTestSuite) SetupTestSuite(serverPort, clientAddr string, mockClients map[string]client.Client) {
	serverAddr := "0.0.0.0:" + serverPort
	log.Printf("启动测试套件，服务器地址：%s, 客户端地址：%s\n", serverAddr, clientAddr)

	s.serverAddr = serverAddr
	s.clientAddr = clientAddr
	s.mockClients = mockClients

	// 加载配置
	dir, err := os.Getwd()
	s.Require().NoError(err)
	f, err := os.Open(dir + "/../../../../config/config.yaml")
	s.Require().NoError(err)
	err = econf.LoadFromReader(f, yaml.Unmarshal)
	s.Require().NoError(err)

	// 设置客户端配置
	econf.Set("server.grpc.port", serverPort)
	econf.Set("client", map[string]any{
		"addr":  clientAddr,
		"debug": true,
	})

	log.Printf("config = %s\n", econf.RawConfig())

	// 初始化数据库
	s.db = testioc.InitDBAndTables()

	// 创建服务器
	s.server = ego.New()
	setupCtx, setupCancelFunc := context.WithCancel(context.Background())

	go func() {
		// 使用指定的mock客户端创建应用
		s.app = platformioc.InitGrpcServer(s.mockClients)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.app.StartTasks(ctx)

		// 初始化trace
		tp := prodioc.InitZipkinTracer()
		defer func(tp *trace.TracerProvider, ctx context.Context) {
			err := tp.Shutdown(ctx)
			if err != nil {
				elog.Error("Shutdown zipkinTracer", elog.FieldErr(err))
			}
		}(tp, ctx)

		// 设置服务器地址
		econf.Set("server.grpc", map[string]any{
			"addr": serverPort,
		})

		// 启动服务
		if err := s.server.Serve(
			egovernor.Load("server.governor").Build(),
			func() server.Server {
				setupCancelFunc()
				return s.app.GrpcServer
			}(),
		).Cron(s.app.Crons...).Run(); err != nil {
			elog.Panic("startup", elog.FieldErr(err))
		}
	}()

	// 等待服务启动
	log.Printf("等待服务启动...\n")
	select {
	case <-setupCtx.Done():
		time.Sleep(1 * time.Second)
	case <-time.After(10 * time.Second):
		s.Fail("服务启动超时")
	}

	// 创建客户端
	conn := egrpc.Load("client").Build()
	s.client = notificationv1.NewNotificationServiceClient(conn)
	s.queryClient = notificationv1.NewNotificationQueryServiceClient(conn)
}

// 关闭测试环境
func (s *BaseGRPCServerTestSuite) TearDownTestSuite() {
	log.Printf("关闭测试套件，服务器地址：%s\n", s.serverAddr)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	s.NoError(s.server.Stop(ctx, false))
}

// 准备测试数据
func (s *BaseGRPCServerTestSuite) prepareTemplateData() int64 {
	templateID := time.Now().UnixNano()
	ctx := context.Background()

	// 创建模板 - 直接使用服务层
	template := domain.ChannelTemplate{
		ID:      templateID,
		BizID:   1,
		Channel: domain.ChannelSMS,
		Name:    "Test Template",
		ProviderInfo: domain.ProviderTemplateInfo{
			TemplateID:   "prov-tpl-001",
			ProviderName: "mock-provider",
		},
		Content:   "您的验证码是：${code}",
		Status:    domain.TemplateStatusApproved,
		VersionID: 1,
	}

	// 使用服务层创建模板
	_, err := s.app.TemplateSvc.CreateTemplate(ctx, template)
	s.NoError(err)

	// 创建供应商
	provider := domain.Provider{
		Name:    "mock-provider",
		Channel: domain.ChannelSMS,
		Config: map[string]string{
			"access_key": "mock-key",
			"secret":     "mock-secret",
		},
		Status: domain.ProviderStatusEnabled,
	}

	// 使用服务层创建供应商
	_, err = s.app.ProviderSvc.Create(ctx, provider)
	s.NoError(err)

	// 设置业务配置
	config := domain.BusinessConfig{
		BizID:       1,
		ConfigKey:   "quota.daily.sms",
		ConfigValue: "1000",
	}

	// 使用服务层设置配置
	err = s.app.ConfigSvc.Set(ctx, config)
	s.NoError(err)

	return templateID
}

// 添加JWT认证到context
func (s *BaseGRPCServerTestSuite) contextWithJWT(ctx context.Context) context.Context {
	md := metadata.New(map[string]string{
		"Authorization": "Bearer mock-token-for-biz-id-1",
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// SendNotification测试 - 成功场景
func (s *SuccessGRPCServerTestSuite) TestSendNotification() {
	// 准备测试数据
	templateID := s.prepareTemplateData()

	// 测试用例
	testCases := []struct {
		name          string
		req           *notificationv1.SendNotificationRequest
		setupContext  func(context.Context) context.Context
		wantResp      *notificationv1.SendNotificationResponse
		errAssertFunc assert.ErrorAssertionFunc
	}{
		{
			name: "成功发送短信通知",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        "test-key-1",
					Receivers:  []string{"13800138000"},
					Channel:    notificationv1.Channel_SMS,
					TemplateId: strconv.FormatInt(templateID, 10),
					TemplateParams: map[string]string{
						"code": "123456",
					},
				},
			},
			setupContext: s.contextWithJWT,
			wantResp: &notificationv1.SendNotificationResponse{
				Status: notificationv1.SendStatus_SUCCEEDED,
			},
			errAssertFunc: assert.NoError,
		},
		{
			name: "JWT认证失败",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        "test-key-2",
					Receivers:  []string{"13800138000"},
					Channel:    notificationv1.Channel_SMS,
					TemplateId: strconv.FormatInt(templateID, 10),
					TemplateParams: map[string]string{
						"code": "123456",
					},
				},
			},
			setupContext: func(ctx context.Context) context.Context {
				// 不添加JWT认证信息
				return ctx
			},
			wantResp: &notificationv1.SendNotificationResponse{
				Status:    notificationv1.SendStatus_FAILED,
				ErrorCode: notificationv1.ErrorCode_BIZ_ID_NOT_FOUND,
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，而不是通过错误
		},
		{
			name: "空notification参数",
			req: &notificationv1.SendNotificationRequest{
				Notification: nil,
			},
			setupContext: s.contextWithJWT,
			wantResp: &notificationv1.SendNotificationResponse{
				Status:    notificationv1.SendStatus_FAILED,
				ErrorCode: notificationv1.ErrorCode_INVALID_PARAMETER,
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，而不是通过错误
		},
		{
			name: "无效的模板ID",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        "test-key-3",
					Receivers:  []string{"13800138000"},
					Channel:    notificationv1.Channel_SMS,
					TemplateId: "invalid-id", // 使用非数字模板ID
					TemplateParams: map[string]string{
						"code": "123456",
					},
				},
			},
			setupContext: s.contextWithJWT,
			wantResp: &notificationv1.SendNotificationResponse{
				Status:    notificationv1.SendStatus_FAILED,
				ErrorCode: notificationv1.ErrorCode_INVALID_PARAMETER,
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，而不是通过错误
		},
		{
			name: "未知的渠道类型",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        "test-key-4",
					Receivers:  []string{"13800138000"},
					Channel:    notificationv1.Channel_CHANNEL_UNSPECIFIED, // 使用未指定渠道
					TemplateId: strconv.FormatInt(templateID, 10),
					TemplateParams: map[string]string{
						"code": "123456",
					},
				},
			},
			setupContext: s.contextWithJWT,
			wantResp: &notificationv1.SendNotificationResponse{
				Status:    notificationv1.SendStatus_FAILED,
				ErrorCode: notificationv1.ErrorCode_UNKNOWN_CHANNEL,
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，而不是通过错误
		},
	}

	for _, tt := range testCases {
		tt := tt
		s.T().Run(tt.name, func(t *testing.T) {
			// 添加JWT认证或其他上下文设置
			ctx := tt.setupContext(t.Context())

			// 调用服务
			resp, err := s.client.SendNotification(ctx, tt.req)
			tt.errAssertFunc(t, err)

			if err != nil {
				return
			}

			diff := cmp.Diff(tt.wantResp, resp, protocmp.Transform())
			assert.Empty(t, diff, "diff:\n%s", diff)

			// 验证响应
			// if tt.wantResp.Status != 0 {
			// 	assert.Equal(t, tt.wantResp.Status, resp.Status, "状态应匹配")
			// }
			// if tt.wantResp.ErrorCode != 0 {
			// 	assert.Equal(t, tt.wantResp.ErrorCode, resp.ErrorCode, "错误码应匹配")
			// }
			// if tt.wantResp.Status == notificationv1.SendStatus_SUCCEEDED {
			// 	assert.NotZero(t, resp.NotificationId, "成功时应返回非零通知ID")
			// }
		})
	}
}

// SendNotification测试 - 失败场景
func (s *FailureGRPCServerTestSuite) TestSendNotification() {
	// 准备测试数据
	templateID := s.prepareTemplateData()

	// 测试用例
	testCases := []struct {
		name          string
		req           *notificationv1.SendNotificationRequest
		wantResp      *notificationv1.SendNotificationResponse
		errAssertFunc assert.ErrorAssertionFunc
	}{
		{
			name: "发送通知失败",
			req: &notificationv1.SendNotificationRequest{
				Notification: &notificationv1.Notification{
					Key:        "test-key-5",
					Receivers:  []string{"13800138000"},
					Channel:    notificationv1.Channel_SMS,
					TemplateId: strconv.FormatInt(templateID, 10),
					TemplateParams: map[string]string{
						"code": "123456",
					},
				},
			},
			wantResp: &notificationv1.SendNotificationResponse{
				Status:    notificationv1.SendStatus_FAILED,
				ErrorCode: notificationv1.ErrorCode_SEND_NOTIFICATION_FAILED,
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，不是通过gRPC错误
		},
	}

	for _, tt := range testCases {
		tt := tt
		s.T().Run(tt.name, func(t *testing.T) {
			// 添加JWT认证
			ctx := s.contextWithJWT(t.Context())

			// 调用服务
			resp, err := s.client.SendNotification(ctx, tt.req)
			tt.errAssertFunc(t, err)

			if err != nil {
				return
			}

			diff := cmp.Diff(tt.wantResp, resp, protocmp.Transform())
			assert.Empty(t, diff, "diff:\n%s", diff)

			// 验证响应
			// assert.Equal(t, tt.wantResp.Status, resp.Status, "状态应匹配")
			// assert.Equal(t, tt.wantResp.ErrorCode, resp.ErrorCode, "错误码应匹配")
			// assert.NotEmpty(t, resp.ErrorMessage, "错误信息不应为空")
		})
	}
}
