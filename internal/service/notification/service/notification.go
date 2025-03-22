package service

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
)

type NotificationService interface {
	CreateNotification(ctx context.Context, key string) (domain.Notification, error)
}
