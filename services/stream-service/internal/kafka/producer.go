// Package kafka gives stream-service a producer for the one event
// notification-service needs from it: a stream going live. Everything
// else in stream-service is plain REST/WebSocket -- this is
// intentionally the only Kafka producer in the service, added for this
// one cross-service notification, not a general event-sourcing layer.
package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

const Topic = "stream-events"

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Topic:                  Topic,
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireAll,
		},
	}
}

func (p *Producer) Close() error { return p.writer.Close() }

func (p *Producer) Produce(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}
