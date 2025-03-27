package config

import (
	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
)

type (
	Module struct {
		ignoredInitTablesErr error
		Svc                  service.BusinessConfigService
	}
	BusinessConfig = domain.BusinessConfig
	Service = service.BusinessConfigService
)
