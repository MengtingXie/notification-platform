package ratelimit

import "gitee.com/flycash/notification-platform/internal/domain"

const (
	eventName = "request_rate_limited_events"
)

type RequestRateLimitedEvent struct {
	HandlerName   string                `json:"handlerName"`
	Notifications []domain.Notification `json:"notifications"`
}
