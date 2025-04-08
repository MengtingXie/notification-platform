//go:build e2e

package integration

import (
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	"gitee.com/flycash/notification-platform/internal/service/backup/internal/executor/internal/integration/startup"
	"testing"

	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	"github.com/stretchr/testify/suite"
)

func TestExecutorSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutorTestSuite))
}

type ExecutorTestSuite struct {
	suite.Suite
}

func (s *ExecutorTestSuite) newExecutorService(
	notificationSvc notificationsvc.Service,
	configSvc configsvc.Service,
	providerSvc providersvc.Service,
	templateSvc templatesvc.Service,
	smsClients map[string]sms.Client,
) notificationsvc.ExecutorService {
	return startup.InitService(notificationSvc, configSvc, providerSvc, templateSvc, smsClients)
}

func (s *ExecutorTestSuite) TestSendNotification() {
}
