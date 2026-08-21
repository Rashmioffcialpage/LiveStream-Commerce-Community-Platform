// See chat-service/internal/realtime/pubsub.go for the same reasoning:
// live delivery over Redis Pub/Sub (any replica can reach any connected
// user), Postgres is the durable inbox. One channel per user instead of
// per stream.
package realtime

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func New(redisURL string) (*Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Client{rdb: redis.NewClient(opt)}, nil
}

func (c *Client) Close() error                   { return c.rdb.Close() }
func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

func userChannel(userID string) string {
	return fmt.Sprintf("notifications:%s", userID)
}

func (c *Client) Publish(ctx context.Context, userID string, payload []byte) error {
	return c.rdb.Publish(ctx, userChannel(userID), payload).Err()
}

func (c *Client) Subscribe(ctx context.Context, userID string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, userChannel(userID))
}
