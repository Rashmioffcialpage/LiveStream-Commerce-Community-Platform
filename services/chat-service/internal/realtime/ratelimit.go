package realtime

import (
	"context"
	"fmt"
	"time"
)

const (
	rateLimitMaxMessages = 5
	rateLimitWindow      = 10 * time.Second
)

// Allow implements a fixed-window rate limit per (stream, user): up to
// rateLimitMaxMessages messages per rateLimitWindow. A fixed window can
// admit up to 2x the limit across a window boundary (e.g. 5 messages at
// 9.9s and 5 more at 10.1s) -- a true sliding window would need a sorted
// set of timestamps instead of one counter. Acceptable here: this is an
// abuse guard against spam bots, not a precise quota, and the boundary
// case only doubles a 10-second burst, not a sustained rate.
func (c *Client) Allow(ctx context.Context, streamID, userID string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s:%s", streamID, userID)
	n, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		c.rdb.Expire(ctx, key, rateLimitWindow)
	}
	return n <= rateLimitMaxMessages, nil
}
