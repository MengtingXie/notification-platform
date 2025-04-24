package ratelimit

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/pkg/mqx"
	"github.com/ecodeclub/mq-api"
)

//go:generate mockgen -source=./producer.go -package=evtmocks -destination=../mocks/ratelimit_event_producer.mock.go -typed RequestRateLimitedEventProducer
type RequestRateLimitedEventProducer interface {
	Produce(ctx context.Context, evt RequestRateLimitedEvent) error
}

func NewRequestRateLimitedEventProducer(q mq.MQ) (RequestRateLimitedEventProducer, error) {
	return mqx.NewGeneralProducer[RequestRateLimitedEvent](q, eventName)
}
