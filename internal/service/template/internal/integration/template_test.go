//go:build e2e

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	auditmocks "gitee.com/flycash/notification-platform/internal/service/audit/mocks"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	providermocks "gitee.com/flycash/notification-platform/internal/service/provider/mocks"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/integration/startup"
	"gitee.com/flycash/notification-platform/internal/service/template/internal/service"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

func TestTemplateServiceSuite(t *testing.T) {
	suite.Run(t, new(TemplateServiceTestSuite))
}

type TemplateServiceTestSuite struct {
	suite.Suite
	db *egorm.Component
}

func (s *TemplateServiceTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
}

func (s *TemplateServiceTestSuite) TearDownTest() {
	// 每个测试后清空表数据
	s.db.Exec("TRUNCATE TABLE `channel_templates`")
	s.db.Exec("TRUNCATE TABLE `channel_template_versions`")
	s.db.Exec("TRUNCATE TABLE `channel_template_providers`")
}

func (s *TemplateServiceTestSuite) newChannelTemplateService(ctrl *gomock.Controller) (templatesvc.Service, *providermocks.MockProviderService, *auditmocks.MockAuditService) {
	mockProviderSvc := providermocks.NewMockProviderService(ctrl)
	mockAuditSvc := auditmocks.NewMockAuditService(ctrl)
	return startup.InitChannelTemplateService(mockProviderSvc, mockAuditSvc), mockProviderSvc, mockAuditSvc
}

// 创建测试用的模板对象
func (s *TemplateServiceTestSuite) createTestTemplate(ownerID int64, ownerType domain.OwnerType, channel domain.Channel) domain.ChannelTemplate {
	now := time.Now()
	return domain.ChannelTemplate{
		OwnerID:      ownerID,
		OwnerType:    ownerType,
		Name:         fmt.Sprintf("测试模板-%d", now.Unix()),
		Description:  "这是一个测试模板",
		Channel:      channel,
		BusinessType: domain.BusinessTypeNotification,
	}
}

func (s *TemplateServiceTestSuite) TestCreateTemplate() {
	t := s.T()

	mockProviders := s.mockProviders()

	tests := []struct {
		name          string
		template      domain.ChannelTemplate
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name: "SMS_营销_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "SMS_通知_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(2, domain.OwnerTypePerson, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "SMS_验证码_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(3, domain.OwnerTypePerson, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "SMS_营销_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(4, domain.OwnerTypeOrganization, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "SMS_通知_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(5, domain.OwnerTypeOrganization, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "SMS_验证码_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(6, domain.OwnerTypeOrganization, domain.ChannelSMS)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_营销_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(7, domain.OwnerTypePerson, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_通知_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(8, domain.OwnerTypePerson, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_验证码_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(9, domain.OwnerTypePerson, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_营销_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(10, domain.OwnerTypeOrganization, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_通知_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(11, domain.OwnerTypeOrganization, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "EMAIL_验证码_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(12, domain.OwnerTypeOrganization, domain.ChannelEmail)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_营销_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(13, domain.OwnerTypePerson, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_通知_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(14, domain.OwnerTypePerson, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_验证码_个人",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(15, domain.OwnerTypePerson, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_营销_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(16, domain.OwnerTypeOrganization, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypePromotion
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_通知_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(17, domain.OwnerTypeOrganization, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypeNotification
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
		{
			name: "INAPP_验证码_组织",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(18, domain.OwnerTypeOrganization, domain.ChannelInApp)
				t.BusinessType = domain.BusinessTypeVerificationCode
				return t
			}(),
			assertErrFunc: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc, mockProviderSvc, _ := s.newChannelTemplateService(ctrl)

			var providers []providersvc.Provider
			switch tt.template.Channel {
			case domain.ChannelSMS:
				providers = []providersvc.Provider{mockProviders[0], mockProviders[3]}
			case domain.ChannelEmail:
				providers = []providersvc.Provider{mockProviders[1], mockProviders[2]}
			case domain.ChannelInApp:
				providers = []providersvc.Provider{mockProviders[4], mockProviders[5]}
			}
			mockProviderSvc.EXPECT().
				GetProvidersByChannel(gomock.Any(), providersvc.Channel(tt.template.Channel)).
				Return(providers, nil)

			createdTemplate, err := svc.CreateTemplate(t.Context(), tt.template)
			tt.assertErrFunc(t, err)

			if err != nil {
				return
			}

			// 验证创建的版本信息
			require.Len(t, createdTemplate.Versions, 1)
			createdVersion := createdTemplate.Versions[0]

			tt.template.Versions = []domain.ChannelTemplateVersion{
				{
					ChannelTemplateID: createdTemplate.ID,
					Name:              "版本名称，比如v1.0.0",
					Signature:         "提前配置好的可用的短信签名或者Email收件人",
					Content:           "模版变量使用${code}格式，也可以没有变量",
					Remark:            "模版使用场景或者用途说明，有利于供应商审核通过",
					AuditStatus:       domain.AuditStatusPending,
					Providers: func() []domain.ChannelTemplateProvider {
						channelTemplateProviders := make([]domain.ChannelTemplateProvider, len(providers))
						for i := range providers {
							channelTemplateProviders[i] = domain.ChannelTemplateProvider{
								TemplateID:        createdTemplate.ID,
								TemplateVersionID: createdVersion.ID,
								ProviderID:        providers[i].ID,
								ProviderName:      providers[i].Name,
								AuditStatus:       domain.AuditStatusPending,
							}
						}
						return channelTemplateProviders
					}(),
				},
			}
			s.assertTemplate(t, tt.template, createdTemplate)
		})
	}
}

func (s *TemplateServiceTestSuite) mockProviders() []providersvc.Provider {
	return []providersvc.Provider{
		{
			ID:      1,
			Name:    "测试供应商1",
			Code:    "test-provider-1",
			Channel: providersvc.ChannelSMS,
		},
		{
			ID:      2,
			Name:    "测试供应商2",
			Code:    "test-provider-2",
			Channel: providersvc.ChannelEmail,
		},
		{
			ID:      3,
			Name:    "测试供应商1",
			Code:    "test-provider-1",
			Channel: providersvc.ChannelEmail,
		},
		{
			ID:      4,
			Name:    "测试供应商2",
			Code:    "test-provider-2",
			Channel: providersvc.ChannelSMS,
		},
		{
			ID:      5,
			Name:    "测试供应商2",
			Code:    "test-provider-2",
			Channel: providersvc.ChannelInApp,
		},
		{
			ID:      6,
			Name:    "测试供应商1",
			Code:    "test-provider-1",
			Channel: providersvc.ChannelInApp,
		},
	}
}

func (s *TemplateServiceTestSuite) assertTemplate(t *testing.T, expected, actual domain.ChannelTemplate) {
	t.Helper()

	s.assertTemplateExcludeVersions(t, expected, actual)

	assert.Equal(t, len(expected.Versions), len(actual.Versions))
	for i := range expected.Versions {
		s.assertTemplateVersion(t, expected.Versions[i], actual.Versions[i])
	}
}

func (s *TemplateServiceTestSuite) assertTemplateExcludeVersions(t *testing.T, expected domain.ChannelTemplate, actual domain.ChannelTemplate) {
	t.Helper()
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
}

func (s *TemplateServiceTestSuite) assertTemplateVersion(t *testing.T, expected, actual domain.ChannelTemplateVersion) {
	t.Helper()

	s.assertTemplateVersionExcludeProviders(t, expected, actual)

	assert.Equal(t, len(expected.Providers), len(actual.Providers))
	for i := range expected.Providers {
		s.assertTemplateProvider(t, expected.Providers[i], actual.Providers[i])
	}
}

func (s *TemplateServiceTestSuite) assertTemplateVersionExcludeProviders(t *testing.T, expected domain.ChannelTemplateVersion, actual domain.ChannelTemplateVersion) {
	t.Helper()
	assert.NotZero(t, actual.ID)
	assert.Equal(t, expected.ChannelTemplateID, actual.ChannelTemplateID)
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
}

func (s *TemplateServiceTestSuite) assertTemplateProvider(t *testing.T, expected, actual domain.ChannelTemplateProvider) {
	t.Helper()
	assert.NotZero(t, actual.ID)
	assert.Equal(t, expected.TemplateID, actual.TemplateID)
	assert.Equal(t, expected.TemplateVersionID, actual.TemplateVersionID)
	assert.Equal(t, expected.ProviderID, actual.ProviderID)
	assert.Equal(t, expected.ProviderName, actual.ProviderName)
	assert.Equal(t, expected.RequestID, actual.RequestID)
	assert.Equal(t, expected.ProviderTemplateID, actual.ProviderTemplateID)
	assert.Equal(t, expected.AuditStatus, actual.AuditStatus)
	assert.Equal(t, expected.RejectReason, actual.RejectReason)
	assert.Equal(t, expected.LastReviewSubmissionTime, actual.LastReviewSubmissionTime)
	assert.NotZero(t, actual.Ctime)
	assert.NotZero(t, actual.Utime)
}

func (s *TemplateServiceTestSuite) TestCreateTemplateFailed() {
	t := s.T()

	tests := []struct {
		name          string
		mockProviders []providersvc.Provider
		template      domain.ChannelTemplate
		assertErrFunc assert.ErrorAssertionFunc
	}{
		{
			name: "OwnerID非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.OwnerID = 0
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name: "OwnerType非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.OwnerType = ""
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name: "Name非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.Name = ""
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name: "Channel非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.Channel = ""
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name: "Description非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.Description = ""
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name: "BusinessType非法",
			template: func() domain.ChannelTemplate {
				t := s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS)
				t.BusinessType = 0
				return t
			}(),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrInvalidParameter)
			},
		},
		{
			name:          "无可用供应商",
			mockProviders: []providersvc.Provider{},
			template:      s.createTestTemplate(1, domain.OwnerTypePerson, domain.ChannelSMS),
			assertErrFunc: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, service.ErrCreateTemplateFailed)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc, mockProviderSvc, _ := s.newChannelTemplateService(ctrl)

			if tt.mockProviders != nil && tt.template.Channel != "" {
				mockProviderSvc.EXPECT().
					GetProvidersByChannel(gomock.Any(), providersvc.Channel(tt.template.Channel)).
					Return(tt.mockProviders, nil)
			}

			_, err := svc.CreateTemplate(context.Background(), tt.template)

			tt.assertErrFunc(t, err)
		})
	}
}

func (s *TemplateServiceTestSuite) TestGetTemplates() {
	t := s.T()

	mockProviders := s.mockProviders()

	// 先创建一些供测试使用的模板
	setupProvidersAndTemplates := func(t *testing.T, setupInfo struct {
		ownerID   int64
		ownerType domain.OwnerType
		templates []domain.ChannelTemplate
	},
	) []domain.ChannelTemplate {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, mockProviderSvc, _ := s.newChannelTemplateService(ctrl)

		// 设置mock
		mockProviderSvc.EXPECT().
			GetProvidersByChannel(gomock.Any(), providersvc.ChannelSMS).
			Return([]providersvc.Provider{mockProviders[0], mockProviders[3]}, nil).
			AnyTimes()
		mockProviderSvc.EXPECT().
			GetProvidersByChannel(gomock.Any(), providersvc.ChannelEmail).
			Return([]providersvc.Provider{mockProviders[1], mockProviders[2]}, nil).
			AnyTimes()
		mockProviderSvc.EXPECT().
			GetProvidersByChannel(gomock.Any(), providersvc.ChannelInApp).
			Return([]providersvc.Provider{mockProviders[4], mockProviders[5]}, nil).
			AnyTimes()

		// 创建模板
		var createdTemplates []domain.ChannelTemplate
		for _, tmpl := range setupInfo.templates {
			created, err := svc.CreateTemplate(t.Context(), tmpl)
			require.NoError(t, err)
			createdTemplates = append(createdTemplates, created)
		}

		return createdTemplates
	}

	tests := []struct {
		name  string
		setup struct {
			ownerID   int64
			ownerType domain.OwnerType
			templates []domain.ChannelTemplate
		}
		queryOwnerID   int64
		queryOwnerType domain.OwnerType
		expectedCount  int
		assertErrFunc  assert.ErrorAssertionFunc
	}{
		{
			name: "查询存在的多个模板",
			setup: struct {
				ownerID   int64
				ownerType domain.OwnerType
				templates []domain.ChannelTemplate
			}{
				ownerID:   100,
				ownerType: domain.OwnerTypePerson,
				templates: []domain.ChannelTemplate{
					s.createTestTemplate(100, domain.OwnerTypePerson, domain.ChannelSMS),
					s.createTestTemplate(100, domain.OwnerTypePerson, domain.ChannelEmail),
					s.createTestTemplate(100, domain.OwnerTypePerson, domain.ChannelInApp),
				},
			},
			queryOwnerID:   100,
			queryOwnerType: domain.OwnerTypePerson,
			expectedCount:  3,
			assertErrFunc:  assert.NoError,
		},
		{
			name: "查询存在的单个模板",
			setup: struct {
				ownerID   int64
				ownerType domain.OwnerType
				templates []domain.ChannelTemplate
			}{
				ownerID:   101,
				ownerType: domain.OwnerTypeOrganization,
				templates: []domain.ChannelTemplate{
					s.createTestTemplate(101, domain.OwnerTypeOrganization, domain.ChannelSMS),
				},
			},
			queryOwnerID:   101,
			queryOwnerType: domain.OwnerTypeOrganization,
			expectedCount:  1,
			assertErrFunc:  assert.NoError,
		},
		{
			name: "查询不存在的模板",
			setup: struct {
				ownerID   int64
				ownerType domain.OwnerType
				templates []domain.ChannelTemplate
			}{
				ownerID:   102,
				ownerType: domain.OwnerTypePerson,
				templates: []domain.ChannelTemplate{},
			},
			queryOwnerID:   999999,
			queryOwnerType: domain.OwnerTypePerson,
			expectedCount:  0,
			assertErrFunc:  assert.NoError,
		},
		{
			name: "不同拥有者类型",
			setup: struct {
				ownerID   int64
				ownerType domain.OwnerType
				templates []domain.ChannelTemplate
			}{
				ownerID:   103,
				ownerType: domain.OwnerTypePerson,
				templates: []domain.ChannelTemplate{
					s.createTestTemplate(103, domain.OwnerTypePerson, domain.ChannelSMS),
				},
			},
			queryOwnerID:   103,
			queryOwnerType: domain.OwnerTypeOrganization,
			expectedCount:  0,
			assertErrFunc:  assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			createdTemplates := setupProvidersAndTemplates(t, tt.setup)

			svc, _, _ := s.newChannelTemplateService(ctrl)

			foundTemplates, err := svc.GetTemplates(t.Context(), tt.queryOwnerID, tt.queryOwnerType)
			tt.assertErrFunc(t, err)

			assert.Len(t, foundTemplates, tt.expectedCount)

			if tt.expectedCount > 0 {
				// 创建一个映射，用于快速查找原始创建的模板
				originalTemplatesMap := make(map[int64]domain.ChannelTemplate)
				for _, tmpl := range createdTemplates {
					if tmpl.OwnerID == tt.queryOwnerID && tmpl.OwnerType == tt.queryOwnerType {
						originalTemplatesMap[tmpl.ID] = tmpl
					}
				}

				// 验证每个返回的模板
				for _, foundTemplate := range foundTemplates {
					// 确认找到的模板是预期创建的
					originalTemplate, exists := originalTemplatesMap[foundTemplate.ID]
					assert.True(t, exists, "找到了未创建的模板ID: %d", foundTemplate.ID)
					if !exists {
						continue
					}
					// 使用辅助方法验证模板结构
					s.assertTemplate(t, originalTemplate, foundTemplate)
				}
			}
		})
	}
}
