//go:build v1

package notification

import (
	"gitee.com/flycash/notification-platform/internal/repository"
	"github.com/sony/sonyflake"
)

type notificationService struct {
	repo        repository.NotificationRepository
	idGenerator *sonyflake.Sonyflake
}

func NewNotificationService(repo repository.NotificationRepository, idGenerator *sonyflake.Sonyflake) Service {
	return &notificationService{
		repo:        repo,
		idGenerator: idGenerator,
	}
}
