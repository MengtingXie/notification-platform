package ratelimit

import (
	"context"

	mqx "gitee.com/flycash/notification-platform/internal/pkg/mqx2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

//go:generate mockgen -source=./producer.go -package=evtmocks -destination=../mocks/ratelimit_event_producer.mock.go -typed RequestRateLimitedEventProducer
type RequestRateLimitedEventProducer interface {
	Produce(ctx context.Context, evt RequestRateLimitedEvent) error
}

func NewRequestRateLimitedEventProducer(producer *kafka.Producer) (RequestRateLimitedEventProducer, error) {
	return mqx.NewGeneralProducer[RequestRateLimitedEvent](producer, eventName)
}
