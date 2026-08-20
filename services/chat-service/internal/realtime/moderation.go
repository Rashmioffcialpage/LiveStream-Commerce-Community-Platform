package realtime

import (
	"context"
	"fmt"
	"time"
)

// Mutes are TTL'd, not permanent -- closer to a Twitch-style timeout than
// a ban. A creator re-mutes to extend it; letting a mute silently expire
// on its own is the intended behavior, not a bug to guard against.

func muteKey(channelID, userID string) string {
	return fmt.Sprintf("mute:%s:%s", channelID, userID)
}

func (c *Client) Mute(ctx context.Context, channelID, userID string, duration time.Duration) error {
	return c.rdb.Set(ctx, muteKey(channelID, userID), "1", duration).Err()
}

func (c *Client) Unmute(ctx context.Context, channelID, userID string) error {
	return c.rdb.Del(ctx, muteKey(channelID, userID)).Err()
}

func (c *Client) IsMuted(ctx context.Context, channelID, userID string) (bool, error) {
	n, err := c.rdb.Exists(ctx, muteKey(channelID, userID)).Result()
	return n > 0, err
}
