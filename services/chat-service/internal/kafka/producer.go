package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

const Topic = "chat-messages"

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Topic:                  Topic,
			Balancer:               &kafka.Hash{}, // key = stream_id -- all messages for one stream land on the same partition, preserving order
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireAll,
		},
	}
}

func (p *Producer) Close() error { return p.writer.Close() }

// Produce is fire-and-forget from the caller's perspective (called after
// the message has already been broadcast live via Redis Pub/Sub) -- it's
// the durability path, not the latency path. A failure here means a chat
// message won't appear in history, not that it failed to send; logged by
// the caller, not retried inline.
func (p *Producer) Produce(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}
