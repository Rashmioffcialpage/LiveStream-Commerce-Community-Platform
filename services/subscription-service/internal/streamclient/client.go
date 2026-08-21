// Package streamclient is subscription-service's HTTP client to
// stream-service, resolving a channel slug to its id/creator_id -- the
// same "ask the service that owns the data" pattern chat-service's
// streamclient uses.
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

type Channel struct {
	ID        string `json:"id"`
	CreatorID string `json:"creator_id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
}

var ErrNotFound = fmt.Errorf("not found")

func (c *Client) GetChannelBySlug(slug string) (*Channel, error) {
	resp, err := c.http.Get(c.baseURL + "/channels/" + slug)
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
