package audit

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/pkg/mqx"
	"github.com/ecodeclub/mq-api"
)

//go:generate mockgen -source=./audit_result_event_producer.go -package=evtmocks -destination=../mocks/audit.mock.go -typed ResultCallbackEventProducer
type ResultCallbackEventProducer interface {
	Produce(ctx context.Context, evt CallbackResultEvent) error
}

func NewResultCallbackEventProducer(q mq.MQ) (ResultCallbackEventProducer, error) {
	return mqx.NewGeneralProducer[CallbackResultEvent](q, eventName)
}
