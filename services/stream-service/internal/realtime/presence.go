// Package realtime tracks ephemeral live-viewer counts in Redis. This is
// deliberately not in Postgres: it churns on every WebSocket connect/
// disconnect, doesn't need durability past the stream ending, and Redis
// TTLs give us free cleanup if a viewer connection dies without a clean
// close (same reasoning as fraud-detection's Redis-backed velocity
// counters -- hot, ephemeral state doesn't belong in the durable store).
package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Presence struct {
	rdb *redis.Client
}

func NewPresence(redisURL string) (*Presence, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Presence{rdb: redis.NewClient(opt)}, nil
}

func (p *Presence) Close() error { return p.rdb.Close() }

func (p *Presence) Ping(ctx context.Context) error {
	return p.rdb.Ping(ctx).Err()
}

func key(streamID string) string {
	return fmt.Sprintf("stream:%s:viewers", streamID)
}

// Join adds a viewer connection to the stream's presence set and refreshes
// its TTL. connID identifies one WebSocket connection, not one user --
// the same user in two tabs counts as two viewers, matching what a real
// concurrent-viewer count means.
func (p *Presence) Join(ctx context.Context, streamID, connID string) error {
	pipe := p.rdb.TxPipeline()
	pipe.SAdd(ctx, key(streamID), connID)
	pipe.Expire(ctx, key(streamID), 2*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (p *Presence) Leave(ctx context.Context, streamID, connID string) error {
	return p.rdb.SRem(ctx, key(streamID), connID).Err()
}

func (p *Presence) Count(ctx context.Context, streamID string) (int, error) {
	n, err := p.rdb.SCard(ctx, key(streamID)).Result()
	return int(n), err
}
