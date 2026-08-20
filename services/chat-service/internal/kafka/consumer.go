package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"

	"chat-service/internal/model"
)

// RunConsumer persists every chat message from Kafka to Postgres -- the
// durable history a client backfills from on connect/reconnect. Runs in
// its own goroutine for the life of the process; DB.InsertMessage's
// ON CONFLICT DO NOTHING makes a redelivered message (kafka-go's consumer
// group is at-least-once) a no-op rather than a duplicate row.
func RunConsumer(ctx context.Context, brokers, groupID string, insert func(context.Context, model.ChatMessage) error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","),
		Topic:   Topic,
		GroupID: groupID,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("chat consumer: fetch", "err", err)
			continue
		}

		var m model.ChatMessage
		if err := json.Unmarshal(msg.Value, &m); err != nil {
			slog.Error("chat consumer: malformed message, skipping", "err", err)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		if err := insert(ctx, m); err != nil {
			slog.Error("chat consumer: insert failed, will retry on redelivery", "err", err)
			continue // don't commit -- this message will be refetched
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			slog.Error("chat consumer: commit failed", "err", err)
		}
	}
}
