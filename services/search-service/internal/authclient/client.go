// See notification-service/internal/authclient for the same shape and
// rationale: a Kafka event only carries a creator_id, search needs a
// real name to index and match against.
package authclient

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

type user struct {
	DisplayName string `json:"display_name"`
}

func (c *Client) DisplayNameOr(id string) string {
	resp, err := c.http.Get(c.baseURL + "/internal/users/" + id)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "unknown creator"
	}
	defer resp.Body.Close()
	var u user
	if json.NewDecoder(resp.Body).Decode(&u) != nil || u.DisplayName == "" {
		return "unknown creator"
	}
	return u.DisplayName
}
