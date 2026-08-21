package model

import "time"

type ChargeStatus string

const (
	StatusSucceeded ChargeStatus = "succeeded"
	StatusFailed    ChargeStatus = "failed"
)

type Charge struct {
	ID             string       `json:"id"`
	UserID         string       `json:"user_id"`
	AmountCents    int          `json:"amount_cents"`
	Currency       string       `json:"currency"`
	Description    string       `json:"description"`
	Status         ChargeStatus `json:"status"`
	IdempotencyKey string       `json:"idempotency_key"`
	CreatedAt      time.Time    `json:"created_at"`
}
