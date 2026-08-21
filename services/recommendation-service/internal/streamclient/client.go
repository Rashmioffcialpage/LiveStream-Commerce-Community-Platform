// Package streamclient fetches the channel catalog to rank -- the
// recommendation model scores channels it doesn't own any copy of,
// asking stream-service for the current list on every /feed request
// rather than keeping a stale local cache of what channels exist.
package streamclient

import (
	"encoding/json"
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

type Channel struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	CreatorID   string `json:"creator_id"`
}

func (c *Client) ListChannels() ([]Channel, error) {
	resp, err := c.http.Get(c.baseURL + "/channels")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var channels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, err
	}
	return channels, nil
}

// GetChannelCategory resolves a channel_id to its category -- subscription
// and gift events only carry the ID, not the category, unlike view events
// (stream-service already has the category in hand when it emits those).
func (c *Client) GetChannelCategory(channelID string) (string, error) {
	resp, err := c.http.Get(c.baseURL + "/internal/channels/" + channelID)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var ch Channel
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return "", err
	}
	return ch.Category, nil
}
