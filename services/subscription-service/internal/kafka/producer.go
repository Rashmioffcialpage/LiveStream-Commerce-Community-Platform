package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

const Topic = "subscription-events"

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Topic:                  Topic,
			Balancer:               &kafka.Hash{}, // key = channel_id -- events for one channel stay ordered
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireAll,
		},
	}
}

func (p *Producer) Close() error { return p.writer.Close() }

// Produce is fire-and-forget from the subscribe/cancel handler's
// perspective -- the subscription itself is already durably written to
// Postgres by the time this is called; a failure here means
// notification-service (Task 6) won't hear about it promptly, not that
// the subscription failed.
func (p *Producer) Produce(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}

type Event struct {
	Type           string `json:"type"` // "subscribed" | "cancelled"
	SubscriptionID string `json:"subscription_id"`
	SubscriberID   string `json:"subscriber_id"`
	ChannelID      string `json:"channel_id"`
}
