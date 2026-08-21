package kafka

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"
)

// RunConsumer reads one topic and hands each message to handler,
// committing only on success -- same at-least-once-with-idempotent-
// handler shape as chat-service's Kafka consumer. notification-service
// runs three of these concurrently (subscription-events, gift-events,
// stream-events), one per topic, each its own consumer group.
func RunConsumer(ctx context.Context, brokers, topic, groupID string, handler func(context.Context, []byte) error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","),
		Topic:   topic,
		GroupID: groupID,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("notification consumer: fetch", "topic", topic, "err", err)
			continue
		}

		if err := handler(ctx, msg.Value); err != nil {
			slog.Error("notification consumer: handler failed, will retry on redelivery", "topic", topic, "err", err)
			continue // don't commit -- refetched
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			slog.Error("notification consumer: commit failed", "topic", topic, "err", err)
		}
	}
}
