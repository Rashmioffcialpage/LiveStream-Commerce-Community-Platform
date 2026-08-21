// Package kafka gives stream-service a producer for the two events other
// services need from it: a stream going live (notification-service) and
// channels/streams being created (search-service, indexing them for
// search). Everything else in stream-service is plain REST/WebSocket --
// this is intentionally scoped to these cross-service notifications, not
// a general event-sourcing layer.
package kafka

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

const (
	TopicStream  = "stream-events"
	TopicChannel = "channel-events"
)

// Producer has no fixed topic -- stream-service publishes to two
// different topics from one process, so each Produce call names its own
// destination rather than the Writer being locked to one at construction.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireAll,
		},
	}
}

func (p *Producer) Close() error { return p.writer.Close() }

func (p *Producer) Produce(ctx context.Context, topic string, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: key, Value: value})
}
