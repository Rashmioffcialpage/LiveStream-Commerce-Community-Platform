package realtime

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Live message delivery goes over Redis Pub/Sub, not a purely in-process
// hub -- same reasoning as fraud-detection's live-decisions channel: any
// chat-service replica might hold the WebSocket for any given viewer, so
// a message ingested on replica A has to reach viewers connected to
// replica B. Kafka (see internal/kafka) is the durability path; Pub/Sub is
// the low-latency fan-out path. Losing a Pub/Sub message to a disconnect
// mid-broadcast just means that one client re-syncs from /history --
// nothing is silently lost from the record.

func chatChannel(streamID string) string {
	return fmt.Sprintf("chat:%s", streamID)
}

func (c *Client) Publish(ctx context.Context, streamID string, payload []byte) error {
	return c.rdb.Publish(ctx, chatChannel(streamID), payload).Err()
}

func (c *Client) Subscribe(ctx context.Context, streamID string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, chatChannel(streamID))
}
