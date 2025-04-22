package ratelimit

const (
	eventName = "request_rate_limited_events"
)

type RequestRateLimitedEvent struct {
	Request any `json:"req"`
}
