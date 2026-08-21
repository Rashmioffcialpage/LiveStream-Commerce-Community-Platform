package model

import "time"

type Channel struct {
	ID          string    `json:"id"`
	CreatorID   string    `json:"creator_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

type StreamStatus string

const (
	StatusScheduled StreamStatus = "scheduled"
	StatusLive      StreamStatus = "live"
	StatusEnded     StreamStatus = "ended"
)

type Stream struct {
	ID               string       `json:"id"`
	ChannelID        string       `json:"channel_id"`
	Title            string       `json:"title"`
	Tags             []string     `json:"tags"`
	Status           StreamStatus `json:"status"`
	ScheduledStartAt time.Time    `json:"scheduled_start_at"`
	StartedAt        *time.Time   `json:"started_at,omitempty"`
	EndedAt          *time.Time   `json:"ended_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	ViewerCount      int          `json:"viewer_count"`
	RecordingURL     *string      `json:"recording_url,omitempty"`
}
