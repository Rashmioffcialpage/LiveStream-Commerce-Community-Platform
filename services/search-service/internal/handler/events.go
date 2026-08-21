package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"search-service/internal/search"
)

type channelEvent struct {
	Type        string `json:"type"`
	ChannelID   string `json:"channel_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	CreatorID   string `json:"creator_id"`
}

func (h *Handler) HandleChannelEvent(ctx context.Context, raw []byte) error {
	var e channelEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "channel-created" {
		return nil
	}

	doc := search.ChannelDoc{
		ID: e.ChannelID, Slug: e.Slug, Name: e.Name, Category: e.Category,
		Description: e.Description, CreatorID: e.CreatorID,
		CreatorName:  h.Auth.DisplayNameOr(e.CreatorID),
		StreamTitles: []string{}, Tags: []string{},
	}
	if err := h.Search.IndexChannel(ctx, doc); err != nil {
		slog.Error("index channel", "channel_id", e.ChannelID, "err", err)
		return err
	}
	return nil
}

type streamEvent struct {
	Type      string   `json:"type"`
	ChannelID string   `json:"channel_id"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
}

func (h *Handler) HandleStreamEvent(ctx context.Context, raw []byte) error {
	var e streamEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "stream-created" {
		return nil // "stream-started" (notification-service's event) is ignored here
	}

	if err := h.Search.AppendStreamTitle(ctx, e.ChannelID, e.Title, e.Tags); err != nil {
		slog.Error("append stream title", "channel_id", e.ChannelID, "err", err)
		return err
	}
	return nil
}
