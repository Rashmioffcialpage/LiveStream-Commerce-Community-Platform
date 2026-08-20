package model

import "time"

type MessageType string

const (
	TypeMessage  MessageType = "message"
	TypeReaction MessageType = "reaction"
)

type ChatMessage struct {
	ID          string      `json:"id"`
	StreamID    string      `json:"stream_id"`
	UserID      string      `json:"user_id"`
	DisplayName string      `json:"display_name"`
	Type        MessageType `json:"type"`
	Body        string      `json:"body"`
	DeletedAt   *time.Time  `json:"deleted_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}
