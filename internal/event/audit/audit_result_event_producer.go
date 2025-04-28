package audit

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/pkg/mqx"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

//go:generate mockgen -source=./audit_result_event_producer.go -package=evtmocks -destination=../mocks/audit.mock.go -typed ResultCallbackEventProducer
type ResultCallbackEventProducer interface {
	Produce(ctx context.Context, evt CallbackResultEvent) error
}

func NewResultCallbackEventProducer(producer *kafka.Producer) (ResultCallbackEventProducer, error) {
	return mqx.NewGeneralProducer[CallbackResultEvent](producer, eventName)
}
