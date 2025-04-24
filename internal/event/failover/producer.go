package failover

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ecodeclub/mq-api"
)

const (
	FailoverTopic = "fail_over_event"
)

type ConnPoolEvent struct {
	SQL  string `json:"sql"`
	Args []any  `json:"args"`
}

//go:generate mockgen -source=./producer.go -package=evtmocks -destination=../mocks/conn_pool_event_producer.mock.go -typed ConnPoolEventProducer
type ConnPoolEventProducer interface {
	Produce(ctx context.Context, evt ConnPoolEvent) error
}

type Producer struct {
	producer mq.Producer
}

func NewProducer(producer mq.Producer) *Producer {
	return &Producer{producer: producer}
}

func (p *Producer) Produce(ctx context.Context, evt ConnPoolEvent) error {
	evtStr, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化topic的消息失败 %w", err)
	}
	_, err = p.producer.Produce(ctx, &mq.Message{
		Topic: FailoverTopic,
		Value: evtStr,
	})
	return err
}
