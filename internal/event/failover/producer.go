package failover

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
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
	producer *kafka.Producer
}

// NewProducer creates a new Kafka producer with the given configuration
func NewProducer(configMap *kafka.ConfigMap) (*Producer, error) {
	producer, err := kafka.NewProducer(configMap)
	if err != nil {
		return nil, fmt.Errorf("创建Kafka Producer失败: %w", err)
	}
	return &Producer{producer: producer}, nil
}

// Produce serializes and sends a ConnPoolEvent to Kafka
func (p *Producer) Produce(ctx context.Context, evt ConnPoolEvent) error {
	evtStr, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化topic的消息失败 %w", err)
	}
	topic := FailoverTopic

	deliveryChan := make(chan kafka.Event)
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Value: evtStr,
	}, deliveryChan)

	if err != nil {
		return fmt.Errorf("发送消息到Kafka失败: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case e := <-deliveryChan:
		m := e.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			return fmt.Errorf("消息发送失败: %w", m.TopicPartition.Error)
		}
	}
	return nil
}
