package service

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/service/audit/internal/domain"
)

//go:generate mockgen -source=./audit.go -destination=../../mocks/audit.mock.go -package=auditmocks -typed AuditService
type AuditService interface {
	CreateAudit(ctx context.Context, req domain.Audit) (int64, error)
}
