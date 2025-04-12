package audit

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
)

//go:generate mockgen -source=./audit.go -destination=./mocks/audit.mock.go -package=auditmocks -typed Service
type Service interface {
	CreateAudit(ctx context.Context, req domain.Audit) (int64, error)
}

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) CreateAudit(_ context.Context, _ domain.Audit) (int64, error) {
	// TODO implement me
	panic("implement me")
}
