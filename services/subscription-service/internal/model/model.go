package model

import "time"

type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "active"
	StatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID               string             `json:"id"`
	SubscriberID     string             `json:"subscriber_id"`
	ChannelID        string             `json:"channel_id"`
	Status           SubscriptionStatus `json:"status"`
	ChargeID         string             `json:"charge_id"`
	CurrentPeriodEnd time.Time          `json:"current_period_end"`
	CreatedAt        time.Time          `json:"created_at"`
	CancelledAt      *time.Time         `json:"cancelled_at,omitempty"`
}
