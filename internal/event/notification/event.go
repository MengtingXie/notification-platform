package notification

import "gitee.com/flycash/notification-platform/internal/domain"

const (
	eventName = "notification_events"
)

type Event struct {
	Notifications []domain.Notification `json:"notifications"`
}
