// See notification-service/internal/kafka for the identical shape and
// the same fixed this project already hit once: each topic gets its own
// consumer group ID, never shared across readers subscribed to
// different topics.
package kafka

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"
)

func RunConsumer(ctx context.Context, brokers, topic, groupID string, handler func(context.Context, []byte) error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","),
		Topic:   topic,
		GroupID: groupID,
	})
	defer reader.Close()
	slog.Info("search consumer: starting", "topic", topic, "group", groupID, "brokers", brokers)

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("search consumer: fetch", "topic", topic, "err", err)
			continue
		}
		if err := handler(ctx, msg.Value); err != nil {
			slog.Error("search consumer: handler failed, will retry on redelivery", "topic", topic, "err", err)
			continue
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			slog.Error("search consumer: commit failed", "topic", topic, "err", err)
		}
	}
}
