package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"recommendation-service/internal/features"
)

type userEvent struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	Category  string `json:"category"`
}

// HandleUserEvent is the Feature Pipeline's entry point for raw view
// events -- category is already on the payload, no cross-service lookup
// needed.
func (h *Handler) HandleUserEvent(ctx context.Context, raw []byte) error {
	var e userEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "view" || e.UserID == "" {
		return nil
	}
	return h.Features.RecordEvent(ctx, e.UserID, e.Category, features.WeightView)
}

type subscriptionEvent struct {
	Type         string `json:"type"`
	SubscriberID string `json:"subscriber_id"`
	ChannelID    string `json:"channel_id"`
}

func (h *Handler) HandleSubscriptionEvent(ctx context.Context, raw []byte) error {
	var e subscriptionEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "subscribed" || e.SubscriberID == "" {
		return nil
	}
	category, err := h.Streams.GetChannelCategory(e.ChannelID)
	if err != nil {
		return err // retryable -- stream-service might be briefly unreachable
	}
	if err := h.Features.RecordEvent(ctx, e.SubscriberID, category, features.WeightSubscribed); err != nil {
		slog.Error("record subscription affinity", "err", err)
		return err
	}
	return nil
}

type giftEvent struct {
	Type      string `json:"type"`
	SenderID  string `json:"sender_id"`
	ChannelID string `json:"channel_id"`
}

func (h *Handler) HandleGiftEvent(ctx context.Context, raw []byte) error {
	var e giftEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "gift" || e.SenderID == "" {
		return nil
	}
	category, err := h.Streams.GetChannelCategory(e.ChannelID)
	if err != nil {
		return err
	}
	if err := h.Features.RecordEvent(ctx, e.SenderID, category, features.WeightGift); err != nil {
		slog.Error("record gift affinity", "err", err)
		return err
	}
	return nil
}
