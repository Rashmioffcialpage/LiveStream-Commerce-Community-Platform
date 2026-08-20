// Package streamclient is chat-service's HTTP client to stream-service --
// chat has no local copy of streams/channels, it asks the service that
// owns that data. Used at WS-connect time (does this stream exist?) and
// for moderation actions (does the caller actually own this stream's
// channel?), not on the hot per-message path.
package streamclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 3 * time.Second}}
}

type Stream struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Status    string `json:"status"`
}

type Channel struct {
	ID        string `json:"id"`
	CreatorID string `json:"creator_id"`
}

var ErrNotFound = fmt.Errorf("not found")

func (c *Client) GetStream(id string) (*Stream, error) {
	resp, err := c.http.Get(c.baseURL + "/streams/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stream-service returned %d", resp.StatusCode)
	}
	var s Stream
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) GetChannel(id string) (*Channel, error) {
	resp, err := c.http.Get(c.baseURL + "/internal/channels/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stream-service returned %d", resp.StatusCode)
	}
	var ch Channel
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// OwnsStream resolves stream -> channel -> creator_id in two calls and
// compares against userID. Used to authorize moderation actions (mute,
// delete message) without chat-service keeping its own copy of ownership.
func (c *Client) OwnsStream(streamID, userID string) (bool, error) {
	stream, err := c.GetStream(streamID)
	if err != nil {
		return false, err
	}
	channel, err := c.GetChannel(stream.ChannelID)
	if err != nil {
		return false, err
	}
	return channel.CreatorID == userID, nil
}
