package mqx2

import (
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

//go:generate mockgen -source=./general_consumer.go -package=evtmocks -destination=../../event/mocks/kafka_consumer.mock.go -typed Consumer
type Consumer interface {
	ReadMessage(timeout time.Duration) (*kafka.Message, error)
	Pause(partitions []kafka.TopicPartition) (err error)
	Resume(partitions []kafka.TopicPartition) (err error)
	Poll(timeoutMs int) (event kafka.Event)
	CommitMessage(m *kafka.Message) ([]kafka.TopicPartition, error)
}
