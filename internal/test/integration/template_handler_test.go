//go:build e2e

package integration

import (
	"net/http"
	"testing"

	"gitee.com/flycash/notification-platform/internal/domain"
	auditmocks "gitee.com/flycash/notification-platform/internal/service/audit/mocks"
	providermocks "gitee.com/flycash/notification-platform/internal/service/provider/mocks"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
	smsmocks "gitee.com/flycash/notification-platform/internal/service/provider/sms/client/mocks"
	"gitee.com/flycash/notification-platform/internal/test"
	templateioc "gitee.com/flycash/notification-platform/internal/test/integration/ioc/template"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	templateweb "gitee.com/flycash/notification-platform/internal/web/template"
	"github.com/ecodeclub/ekit/iox"
	"github.com/ego-component/egorm"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/server/egin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

const (
	ownerID   = int64(234)
	ownerType = "person"
)

func TestTemplateHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(TemplateHandlerTestSuite))
}

type TemplateHandlerTestSuite struct {
	suite.Suite
	server *egin.Component
	db     *egorm.Component
}

func (s *TemplateHandlerTestSuite) SetupSuite() {
	s.db = testioc.InitDBAndTables()
}

func (s *TemplateHandlerTestSuite) newService(ctrl *gomock.Controller) (templateSvc *templateioc.Service, providerSvc *providermocks.MockService, auditSvc *auditmocks.MockService, clients map[string]client.Client) {
	mockProviderSvc := providermocks.NewMockService(ctrl)
	mockAuditSvc := auditmocks.NewMockService(ctrl)
	mockClient1 := smsmocks.NewMockClient(ctrl)
	mockClient2 := smsmocks.NewMockClient(ctrl)

	clients = map[string]client.Client{
		"mock-provider-name-1": mockClient1,
		"mock-provider-name-2": mockClient2,
	}
	svc := templateioc.Init(mockProviderSvc, mockAuditSvc, clients)
	return svc, mockProviderSvc, mockAuditSvc, clients
}

func (s *TemplateHandlerTestSuite) TearDownSuite() {
	err := s.db.Exec("DROP TABLE `channel_templates`").Error
	s.NoError(err)
	err = s.db.Exec("DROP TABLE `channel_template_versions`").Error
	s.NoError(err)
	err = s.db.Exec("DROP TABLE `channel_template_providers`").Error
	s.NoError(err)
}

func (s *TemplateHandlerTestSuite) TearDownTest() {
	err := s.db.Exec("TRUNCATE TABLE `channel_templates`").Error
	s.NoError(err)
	err = s.db.Exec("TRUNCATE TABLE `channel_template_versions`").Error
	s.NoError(err)
	err = s.db.Exec("TRUNCATE TABLE `channel_template_providers`").Error
	s.NoError(err)
}

func (s *TemplateHandlerTestSuite) newGinServer(handler *templateweb.Handler) *egin.Component {
	econf.Set("server", map[string]any{"contextTimeout": "1s"})
	server := egin.Load("server").Build()
	handler.PublicRoutes(server.Engine)
	return server
}

func (s *TemplateHandlerTestSuite) TestHandler_ListTemplates() {
	t := s.T()

	testCases := []struct {
		name           string
		newHandlerFunc func(t *testing.T, ctrl *gomock.Controller) *templateweb.Handler
		req            templateweb.ListTemplatesReq
		wantCode       int
		wantResp       test.Result[templateweb.ListTemplatesResp]
	}{
		{
			name: "获取成功",
			newHandlerFunc: func(t *testing.T, ctrl *gomock.Controller) *templateweb.Handler {
				t.Helper()

				svc, providerSvc, _, _ := s.newService(ctrl)

				providerSvc.EXPECT().GetByChannel(gomock.Any(), domain.ChannelSMS).Return([]domain.Provider{
					{
						ID:      1,
						Name:    "mock-provider-name-1",
						Channel: domain.ChannelSMS,
						Status:  domain.ProviderStatusActive,
					},
					{
						ID:      2,
						Name:    "mock-provider-name-2",
						Channel: domain.ChannelSMS,
						Status:  domain.ProviderStatusActive,
					},
				}, nil)

				_, err := svc.Svc.CreateTemplate(t.Context(), domain.ChannelTemplate{
					OwnerID:      ownerID,
					OwnerType:    ownerType,
					Name:         "list-templates-01",
					Description:  "list-templates-desc",
					Channel:      domain.ChannelSMS,
					BusinessType: domain.BusinessTypePromotion,
				})
				require.NoError(t, err)

				handler := templateweb.NewHandler(svc.Svc)
				return handler
			},
			req: templateweb.ListTemplatesReq{
				OwnerID:   ownerID,
				OwnerType: ownerType,
			},
			wantCode: 200,
			wantResp: test.Result[templateweb.ListTemplatesResp]{
				Data: templateweb.ListTemplatesResp{
					Templates: []templateweb.ChannelTemplate{
						{
							OwnerID:      ownerID,
							OwnerType:    ownerType,
							Name:         "list-templates-01",
							Description:  "list-templates-desc",
							Channel:      domain.ChannelSMS.String(),
							BusinessType: domain.BusinessTypePromotion.ToInt64(),
							Versions: []templateweb.ChannelTemplateVersion{
								{
									Name:        "版本名称，比如v1.0.0",
									Signature:   "提前配置好的可用的短信签名或者Email收件人",
									Content:     "模版变量使用${code}格式，也可以没有变量",
									Remark:      "模版使用场景或者用途说明，有利于供应商审核通过",
									AuditStatus: domain.AuditStatusPending.String(),
									Providers: []templateweb.ChannelTemplateProvider{
										{
											ProviderID:      1,
											ProviderName:    "mock-provider-name-1",
											ProviderChannel: domain.ChannelSMS.String(),
											AuditStatus:     domain.AuditStatusPending.String(),
										},
										{
											ProviderID:      2,
											ProviderName:    "mock-provider-name-2",
											ProviderChannel: domain.ChannelSMS.String(),
											AuditStatus:     domain.AuditStatusPending.String(),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			req, err := http.NewRequest(http.MethodPost,
				"/templates/list", iox.NewJSONReader(tc.req))

			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)

			recorder := test.NewJSONResponseRecorder[templateweb.ListTemplatesResp]()

			server := s.newGinServer(tc.newHandlerFunc(t, ctrl))

			server.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantCode, recorder.Code)

			actual := recorder.MustScan()

			assert.Equal(t, len(tc.wantResp.Data.Templates), len(actual.Data.Templates))
			for i := range tc.wantResp.Data.Templates {
				s.assertTemplate(t, tc.wantResp.Data.Templates[i], actual.Data.Templates[i])
			}
		})
	}
}

func (s *TemplateHandlerTestSuite) TestHandler_CreateTemplate() {
	t := s.T()

	testCases := []struct {
		name           string
		newHandlerFunc func(t *testing.T, ctrl *gomock.Controller) *templateweb.Handler
		req            templateweb.CreateTemplateReq
		wantCode       int
		wantResp       test.Result[templateweb.CreateTemplateResp]
	}{
		{
			name: "创建成功",
			newHandlerFunc: func(t *testing.T, ctrl *gomock.Controller) *templateweb.Handler {
				t.Helper()

				svc, providerSvc, _, _ := s.newService(ctrl)

				providerSvc.EXPECT().GetByChannel(gomock.Any(), domain.ChannelSMS).Return([]domain.Provider{
					{
						ID:      1,
						Name:    "mock-provider-name-1",
						Channel: domain.ChannelSMS,
						Status:  domain.ProviderStatusActive,
					},
				}, nil)

				handler := templateweb.NewHandler(svc.Svc)
				return handler
			},
			req: templateweb.CreateTemplateReq{
				OwnerID:      ownerID,
				OwnerType:    ownerType,
				Name:         "create-template-test",
				Description:  "create-template-desc",
				Channel:      domain.ChannelSMS.String(),
				BusinessType: domain.BusinessTypePromotion.ToInt64(),
			},
			wantCode: 200,
			wantResp: test.Result[templateweb.CreateTemplateResp]{
				Data: templateweb.CreateTemplateResp{
					Template: templateweb.ChannelTemplate{
						OwnerID:      ownerID,
						OwnerType:    ownerType,
						Name:         "create-template-test",
						Description:  "create-template-desc",
						Channel:      domain.ChannelSMS.String(),
						BusinessType: domain.BusinessTypePromotion.ToInt64(),
						Versions: []templateweb.ChannelTemplateVersion{
							{
								Name:        "版本名称，比如v1.0.0",
								Signature:   "提前配置好的可用的短信签名或者Email收件人",
								Content:     "模版变量使用${code}格式，也可以没有变量",
								Remark:      "模版使用场景或者用途说明，有利于供应商审核通过",
								AuditStatus: domain.AuditStatusPending.String(),
								Providers: []templateweb.ChannelTemplateProvider{
									{
										ProviderID:      1,
										ProviderName:    "mock-provider-name-1",
										ProviderChannel: domain.ChannelSMS.String(),
										AuditStatus:     domain.AuditStatusPending.String(),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			req, err := http.NewRequest(http.MethodPost,
				"/templates/create", iox.NewJSONReader(tc.req))

			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)

			recorder := test.NewJSONResponseRecorder[templateweb.CreateTemplateResp]()

			server := s.newGinServer(tc.newHandlerFunc(t, ctrl))

			server.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantCode, recorder.Code)

			actual := recorder.MustScan()

			s.assertTemplate(t, tc.wantResp.Data.Template, actual.Data.Template)
		})
	}
}

func (s *TemplateHandlerTestSuite) assertTemplate(t *testing.T, expected templateweb.ChannelTemplate, actual templateweb.ChannelTemplate) {
	assert.NotZero(t, actual.ID)
	assert.Equal(t, expected.OwnerID, actual.OwnerID)
	assert.Equal(t, expected.OwnerType, actual.OwnerType)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Description, actual.Description)
	assert.Equal(t, expected.Channel, actual.Channel)
	assert.Equal(t, expected.BusinessType, actual.BusinessType)
	assert.Equal(t, expected.ActiveVersionID, actual.ActiveVersionID)
	assert.NotZero(t, actual.Ctime)
	assert.NotZero(t, actual.Utime)

	assert.Equal(t, len(expected.Versions), len(actual.Versions))
	for i := range expected.Versions {
		s.assertTemplateVersion(t, expected.Versions[i], actual.Versions[i])
	}
}

func (s *TemplateHandlerTestSuite) assertTemplateVersion(t *testing.T, expected templateweb.ChannelTemplateVersion, actual templateweb.ChannelTemplateVersion) {
	assert.NotZero(t, actual.ID)
	assert.NotZero(t, actual.ChannelTemplateID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Signature, actual.Signature)
	assert.Equal(t, expected.Content, actual.Content)
	assert.Equal(t, expected.Remark, actual.Remark)
	assert.Equal(t, expected.AuditID, actual.AuditID)
	assert.Equal(t, expected.AuditorID, actual.AuditorID)
	assert.Equal(t, expected.AuditTime, actual.AuditTime)
	assert.Equal(t, expected.AuditStatus, actual.AuditStatus)
	assert.Equal(t, expected.RejectReason, actual.RejectReason)
	assert.Equal(t, expected.LastReviewSubmissionTime, actual.LastReviewSubmissionTime)
	assert.NotZero(t, actual.Ctime)
	assert.NotZero(t, actual.Utime)

	assert.Equal(t, len(expected.Providers), len(actual.Providers))
	for i := range expected.Providers {
		s.assertTemplateProvider(t, expected.Providers[i], actual.Providers[i])
	}
}

func (s *TemplateHandlerTestSuite) assertTemplateProvider(t *testing.T, expected templateweb.ChannelTemplateProvider, actual templateweb.ChannelTemplateProvider) {
	assert.NotZero(t, actual.ID)
	assert.NotZero(t, actual.TemplateID)
	assert.NotZero(t, actual.TemplateVersionID)
	assert.Equal(t, expected.ProviderID, actual.ProviderID)
	assert.Equal(t, expected.ProviderName, actual.ProviderName)
	assert.Equal(t, expected.ProviderChannel, actual.ProviderChannel)
	assert.Equal(t, expected.RequestID, actual.RequestID)
	assert.Equal(t, expected.ProviderTemplateID, actual.ProviderTemplateID)
	assert.Equal(t, expected.AuditStatus, actual.AuditStatus)
	assert.Equal(t, expected.RejectReason, actual.RejectReason)
	assert.Equal(t, expected.LastReviewSubmissionTime, actual.LastReviewSubmissionTime)
	assert.NotZero(t, actual.Ctime)
	assert.NotZero(t, actual.Utime)
}
