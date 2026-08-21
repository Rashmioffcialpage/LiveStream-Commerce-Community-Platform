package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

const Topic = "gift-events"

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Topic:                  Topic,
			Balancer:               &kafka.Hash{}, // key = recipient_id -- one creator's gift events stay ordered
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireAll,
		},
	}
}

func (p *Producer) Close() error { return p.writer.Close() }

// Produce is fire-and-forget, same reasoning as subscription-service's
// producer: the gift itself is already durably committed to Postgres by
// the time this is called.
func (p *Producer) Produce(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}
