package integration

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/golang-jwt/jwt/v4"
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
	"gopkg.in/yaml.v2"

	jwtpkg "gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/jwt"
)

func TestGRPCServerWithSuccessMock(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SuccessGRPCServerTestSuite))
}

func TestGRPCServerWithFailureMock(t *testing.T) {
	t.Parallel()
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

	// 模拟Send方法
	mockClient.EXPECT().Send(gomock.Any()).Return(client.SendResp{
		RequestID: "mock-req-id",
		PhoneNumbers: map[string]client.SendRespStatus{
			"13800138000": {
				Code:    "OK",
				Message: "发送成功",
			},
		},
	}, nil).AnyTimes()

	// 模拟CreateTemplate方法
	mockClient.EXPECT().CreateTemplate(gomock.Any()).Return(client.CreateTemplateResp{
		RequestID:  "mock-req-id",
		TemplateID: "prov-tpl-001",
	}, nil).AnyTimes()

	// 模拟QueryTemplateStatus方法
	mockClient.EXPECT().QueryTemplateStatus(gomock.Any()).Return(client.QueryTemplateStatusResp{
		RequestID:   "mock-req-id",
		TemplateID:  "prov-tpl-001",
		AuditStatus: client.AuditStatusApproved,
		Reason:      "",
	}, nil).AnyTimes()

	// 配置参数 - 确保与providers表中的name字段匹配
	clients := map[string]client.Client{"mock-provider-1": mockClient}
	log.Printf("设置成功测试套件 Mock客户端: %+v", clients)

	serverProt := 9004
	clientAddr := fmt.Sprintf("127.0.0.1:%d", serverProt)
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

	// 模拟Send方法
	mockClient.EXPECT().Send(gomock.Any()).Return(client.SendResp{
		RequestID: "mock-req-id",
		PhoneNumbers: map[string]client.SendRespStatus{
			"13800138000": {
				Code:    "ERROR",
				Message: "供应商API错误",
			},
		},
	}, errors.New("供应商API错误")).AnyTimes()

	// 模拟CreateTemplate方法
	mockClient.EXPECT().CreateTemplate(gomock.Any()).Return(client.CreateTemplateResp{
		RequestID:  "mock-req-id",
		TemplateID: "prov-tpl-001",
	}, nil).AnyTimes()

	// 模拟QueryTemplateStatus方法
	mockClient.EXPECT().QueryTemplateStatus(gomock.Any()).Return(client.QueryTemplateStatusResp{
		RequestID:   "mock-req-id",
		TemplateID:  "prov-tpl-001",
		AuditStatus: client.AuditStatusApproved,
		Reason:      "",
	}, nil).AnyTimes()

	// 配置参数 - 确保与providers表中的name字段匹配
	clients := map[string]client.Client{"mock-provider-1": mockClient}
	log.Printf("设置失败测试套件 Mock客户端: %+v", clients)

	serverProt := 9005
	clientAddr := fmt.Sprintf("127.0.0.1:%d", serverProt)
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
func (s *BaseGRPCServerTestSuite) SetupTestSuite(serverPort int, clientAddr string, mockClients map[string]client.Client) {
	serverAddr := fmt.Sprintf("0.0.0.0:%d", serverPort)
	log.Printf("启动测试套件，服务器地址：%s, 客户端地址：%s\n", serverAddr, clientAddr)

	s.serverAddr = serverAddr
	s.clientAddr = clientAddr
	s.mockClients = mockClients

	// 加载配置
	dir, err := os.Getwd()
	s.Require().NoError(err)
	f, err := os.Open(dir + "/../../../config/config.yaml")
	s.Require().NoError(err)
	err = econf.LoadFromReader(f, yaml.Unmarshal)
	s.Require().NoError(err)

	// 设置客户端配置
	econf.Set("server.grpc.port", serverPort)
	econf.Set("client", map[string]any{
		"addr":  clientAddr,
		"debug": true,
	})

	// 初始化数据库
	s.db = testioc.InitDBAndTables()

	// 创建服务器
	s.server = ego.New()
	setupCtx, setupCancelFunc := context.WithCancel(context.Background())

	go func() {
		// 使用指定的mock客户端创建应用
		log.Printf("开始创建应用，传入MockClients: %+v\n", s.mockClients)
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

func (s *BaseGRPCServerTestSuite) TearDownTest() {
	// 每个测试后清空表数据
	s.db.Exec("TRUNCATE TABLE `callback_logs`")
	s.db.Exec("TRUNCATE TABLE `business_configs`")
	s.db.Exec("TRUNCATE TABLE `notifications`")
	s.db.Exec("TRUNCATE TABLE `providers`")
	s.db.Exec("TRUNCATE TABLE `quotas`")
	s.db.Exec("TRUNCATE TABLE `channel_templates`")
	s.db.Exec("TRUNCATE TABLE `channel_template_versions`")
	s.db.Exec("TRUNCATE TABLE `channel_template_providers`")
	s.db.Exec("TRUNCATE TABLE `tx_notifications`")
}

// 准备测试数据
func (s *BaseGRPCServerTestSuite) prepareTemplateData() int64 {
	templateID := time.Now().UnixNano()
	ctx := context.Background()

	// 创建SMS供应商
	provider := domain.Provider{
		ID:         1,
		Name:       "mock-provider-1",
		Channel:    domain.ChannelSMS,
		Endpoint:   "https://mock-sms-api.example.com",
		RegionID:   "cn-hangzhou",
		APIKey:     "mock-key",
		APISecret:  "mock-secret",
		Weight:     10,
		QPSLimit:   100,
		DailyLimit: 10000,
		Status:     "ACTIVE",
	}

	// 使用服务层创建供应商
	_, err := s.app.ProviderSvc.Create(ctx, provider)
	s.NoError(err)

	// 创建EMAIL供应商
	provider = domain.Provider{
		Name:       "mock-provider-1",
		Channel:    domain.ChannelEmail,
		Endpoint:   "https://mock-sms-api.example.com",
		RegionID:   "cn-hangzhou",
		APIKey:     "mock-key",
		APISecret:  "mock-secret",
		Weight:     10,
		QPSLimit:   100,
		DailyLimit: 10000,
		Status:     "ACTIVE",
	}

	// 使用服务层创建供应商
	_, err = s.app.ProviderSvc.Create(ctx, provider)
	s.NoError(err)

	// 3. 创建模板和相关记录
	s.createTemplate(ctx, templateID)

	// 设置业务配置
	config := domain.BusinessConfig{
		ID:        1,
		OwnerID:   1,
		OwnerType: "person",
		ChannelConfig: &domain.ChannelConfig{
			Channels: []domain.ChannelItem{
				{
					Channel:  "SMS",
					Priority: 1,
					Enabled:  true,
				},
			},
		},
		RateLimit: 100,
		Quota: &domain.QuotaConfig{
			Monthly: domain.MonthlyConfig{
				SMS: 1000,
			},
		},
	}

	// 使用服务层设置配置
	err = s.app.ConfigSvc.SaveConfig(ctx, config)
	s.NoError(err)

	// 为其创建配额
	err = s.app.QuotaRepo.CreateOrUpdate(ctx, domain.Quota{
		BizID:   1,
		Quota:   10000,
		Channel: domain.ChannelSMS,
	})
	s.NoError(err)
	err = s.app.QuotaRepo.CreateOrUpdate(ctx, domain.Quota{
		BizID:   1,
		Quota:   10000,
		Channel: domain.ChannelEmail,
	})
	s.NoError(err)

	return templateID
}

func (s *BaseGRPCServerTestSuite) createTemplate(ctx context.Context, templateID int64) {
	now := time.Now().Unix()
	versionID := templateID + 1

	// 1. 创建渠道模板记录
	err := s.db.WithContext(ctx).Exec(`
		INSERT INTO channel_templates 
		(id, owner_id, owner_type, name, description, channel, business_type, active_version_id, ctime, utime) 
		VALUES 
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		templateID, 1, "person", "Test Template", "Template for test", "SMS", 3, versionID, now, now,
	).Error
	s.NoError(err)

	// 2. 创建模板版本记录
	err = s.db.WithContext(ctx).Exec(`
		INSERT INTO channel_template_versions 
		(id, channel_template_id, name, signature, content, remark, audit_status, ctime, utime) 
		VALUES 
		(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		versionID, templateID, "v1.0.0", "Test", "您的验证码是：${code}", "测试用的验证码模板", "APPROVED", now, now,
	).Error
	s.NoError(err)

	// 3. 创建模板版本与供应商的关联
	err = s.db.WithContext(ctx).Exec(`
		INSERT INTO channel_template_providers 
		(template_id, template_version_id, provider_id, provider_name, provider_channel, provider_template_id, audit_status, ctime, utime) 
		VALUES 
		(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		templateID, versionID, 1, "mock-provider-1", "SMS", "prov-tpl-001", "APPROVED", now, now,
	).Error
	s.NoError(err)
}

// 添加JWT认证到context
func (s *BaseGRPCServerTestSuite) contextWithJWT(ctx context.Context) context.Context {
	// 使用项目已有的JWT包创建令牌
	jwtAuth := jwtpkg.NewJwtAuth("test_key")

	// 创建包含业务ID的声明
	claims := jwt.MapClaims{
		"biz_id": float64(1),
	}

	// 使用JWT认证包的Encode方法生成令牌
	tokenString, _ := jwtAuth.Encode(claims)

	// 创建带有授权信息的元数据
	md := metadata.New(map[string]string{
		"Authorization": "Bearer " + tokenString,
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
				Status:         notificationv1.SendStatus_SUCCEEDED,
				NotificationId: 1, // 实际任何非零值都可接受
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
			wantResp:      &notificationv1.SendNotificationResponse{},
			errAssertFunc: assert.Error, // 业务错误通过响应返回，而不是通过错误
		},
		{
			name: "空notification参数",
			req: &notificationv1.SendNotificationRequest{
				Notification: nil,
			},
			setupContext: s.contextWithJWT,
			wantResp: &notificationv1.SendNotificationResponse{
				Status:       notificationv1.SendStatus_FAILED,
				ErrorCode:    notificationv1.ErrorCode_INVALID_PARAMETER,
				ErrorMessage: "参数错误: 通知信息不能为空",
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
				Status:       notificationv1.SendStatus_FAILED,
				ErrorCode:    notificationv1.ErrorCode_INVALID_PARAMETER,
				ErrorMessage: "参数错误: 模板ID: invalid-id",
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
				Status:       notificationv1.SendStatus_FAILED,
				ErrorCode:    notificationv1.ErrorCode_INVALID_PARAMETER,
				ErrorMessage: "未知渠道类型",
			},
			errAssertFunc: assert.NoError, // 业务错误通过响应返回，而不是通过错误
		},
	}

	for _, tt := range testCases {
		s.T().Run(tt.name, func(t *testing.T) {
			// 添加JWT认证或其他上下文设置
			ctx := tt.setupContext(t.Context())

			// 调用服务
			log.Printf("发送请求: %+v", tt.req)
			resp, err := s.client.SendNotification(ctx, tt.req)

			if err != nil {
				log.Printf("发送请求失败: %v", err)
			} else {
				log.Printf("接收响应: %+v", resp)
			}

			tt.errAssertFunc(t, err)

			if err != nil {
				return
			}

			if tt.wantResp.Status == notificationv1.SendStatus_SUCCEEDED {
				assert.Greater(t, resp.NotificationId, uint64(0), "成功响应应返回非零通知ID")
			} else {
				assert.Equal(t, tt.wantResp.NotificationId, resp.NotificationId, "通知ID应匹配")
			}
			assert.Equal(t, tt.wantResp.Status, resp.Status, "响应状态应匹配")
			assert.Equal(t, tt.wantResp.ErrorCode, resp.ErrorCode, "错误码应匹配")
			assert.Equal(t, tt.wantResp.ErrorMessage, resp.ErrorMessage, "错误消息应匹配")
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
		s.T().Run(tt.name, func(t *testing.T) {
			// 添加JWT认证
			ctx := s.contextWithJWT(t.Context())

			// 调用服务
			log.Printf("失败测试 - 发送请求: %+v", tt.req)
			resp, err := s.client.SendNotification(ctx, tt.req)

			if err != nil {
				log.Printf("失败测试 - 发送请求失败: %v", err)
			} else {
				log.Printf("失败测试 - 接收响应: %+v", resp)
			}

			tt.errAssertFunc(t, err)

			if err != nil {
				return
			}

			assert.Equal(t, tt.wantResp.NotificationId, resp.NotificationId, "通知ID应匹配")
			assert.Equal(t, tt.wantResp.Status, resp.Status, "响应状态应匹配")
			assert.Equal(t, tt.wantResp.ErrorCode, resp.ErrorCode, "错误码应匹配")
			assert.Equal(t, tt.wantResp.ErrorMessage, resp.ErrorMessage, "错误消息应匹配")
		})
	}
}
