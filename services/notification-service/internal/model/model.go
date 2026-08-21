package model

import "time"

type NotificationType string

const (
	TypeNewSubscriber NotificationType = "new_subscriber"
	TypeGiftReceived  NotificationType = "gift_received"
	TypeStreamStarted NotificationType = "stream_started"
)

type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Body      string           `json:"body"`
	ReadAt    *time.Time       `json:"read_at,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}
