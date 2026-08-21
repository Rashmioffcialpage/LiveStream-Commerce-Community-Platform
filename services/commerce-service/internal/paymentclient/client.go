// See subscription-service/internal/paymentclient for the same shape and
// rationale: the caller's own bearer token is forwarded to payment-service
// unchanged, so a charge is always attributed to whoever is actually
// authenticated.
package paymentclient

import (
	"bytes"
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

type Charge struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

var ErrDeclined = fmt.Errorf("charge declined")

func (c *Client) Charge(token string, amountCents int, currency, description, idempotencyKey string) (*Charge, error) {
	body, _ := json.Marshal(map[string]any{
		"amount_cents":    amountCents,
		"currency":        currency,
		"description":     description,
		"idempotency_key": idempotencyKey,
	})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/charges", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var charge Charge
	if err := json.NewDecoder(resp.Body).Decode(&charge); err != nil {
		return nil, fmt.Errorf("decode charge response: %w", err)
	}
	if resp.StatusCode == http.StatusPaymentRequired {
		return &charge, ErrDeclined
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("payment-service returned %d", resp.StatusCode)
	}
	return &charge, nil
}
