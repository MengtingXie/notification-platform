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
	Sql  string `json:"sql"`
	Args []any  `json:"args"`
}

type ConnPoolEventProducer interface {
	Produce(ctx context.Context, evt ConnPoolEvent) error
}

type Provider struct {
	provider mq.Producer
}

func (p *Provider) Produce(ctx context.Context, evt ConnPoolEvent) error {
	evtStr, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化topic的消息失败 %v", err)
	}
	_, err = p.provider.Produce(ctx, &mq.Message{
		Topic: FailoverTopic,
		Value: evtStr,
	})
	return err
}
