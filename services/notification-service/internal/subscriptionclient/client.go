// Package subscriptionclient fans a "stream started" event out to every
// active subscriber -- subscription-service is the source of truth for
// who's subscribed to a channel, notification-service keeps no copy of
// that relationship.
package subscriptionclient

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
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 5 * time.Second}}
}

type Subscription struct {
	SubscriberID string `json:"subscriber_id"`
}

func (c *Client) ListActiveSubscriberIDs(channelID string) ([]string, error) {
	resp, err := c.http.Get(c.baseURL + "/internal/channels/" + channelID + "/subscribers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription-service returned %d", resp.StatusCode)
	}
	var subs []Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return nil, err
	}
	ids := make([]string, len(subs))
	for i, s := range subs {
		ids[i] = s.SubscriberID
	}
	return ids, nil
}
