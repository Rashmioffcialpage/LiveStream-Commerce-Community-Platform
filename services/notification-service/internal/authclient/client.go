// Package authclient resolves a user_id to a display_name -- a Kafka
// event only ever carries IDs, and a notification body needs a name a
// human can read ("Alice subscribed to your channel", not a UUID).
package authclient

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

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

func (c *Client) GetUser(id string) (*User, error) {
	resp, err := c.http.Get(c.baseURL + "/internal/users/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth-service returned %d", resp.StatusCode)
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// DisplayNameOr resolves id to a display name, falling back to a short
// form of the ID itself if auth-service can't be reached -- a
// notification with a slightly ugly body still beats no notification.
func (c *Client) DisplayNameOr(id string) string {
	u, err := c.GetUser(id)
	if err != nil || u.DisplayName == "" {
		if len(id) > 8 {
			return "user " + id[:8]
		}
		return "a user"
	}
	return u.DisplayName
}
