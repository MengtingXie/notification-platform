package audit

import (
	"gitee.com/flycash/notification-platform/internal/service/audit/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/audit/internal/service"
)

type (
	Module struct {
		Svc Service
	}
	Service      service.AuditService
	Audit        = domain.Audit
	ResourceType = domain.ResourceType
)
