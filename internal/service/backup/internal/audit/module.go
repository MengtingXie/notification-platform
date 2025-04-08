package audit

import (
	"gitee.com/flycash/notification-platform/internal/domain"
	audit2 "gitee.com/flycash/notification-platform/internal/service/audit"
)

type (
	Module struct {
		Svc Service
	}
	Service      audit2.Service
	Audit        = domain.Audit
	ResourceType = domain.ResourceType
)
