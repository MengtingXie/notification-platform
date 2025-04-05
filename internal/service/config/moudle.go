package config

import (
	"gitee.com/flycash/notification-platform/internal/service/config/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/config/internal/service"
)

type (
	Service        service.BusinessConfigService
	BusinessConfig = domain.BusinessConfig
	Module         struct {
		ignoredInitTablesErr error
		Svc                  service.BusinessConfigService
	}
)
