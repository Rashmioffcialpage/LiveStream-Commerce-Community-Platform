package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"notification-service/internal/model"
)

// notify is the common tail of every event handler below: write the
// durable inbox row, push it live to anyone connected right now, and
// "send" the email. Order matters -- the Postgres row exists before
// anything ephemeral (Pub/Sub, email) is attempted, so a crash between
// steps loses at most the live push, never the notification itself; a
// client that missed the live push still sees it on next GET /notifications
// or WebSocket reconnect (which replays recent history, see NotifyWS).
func (h *Handler) notify(ctx context.Context, userID string, typ model.NotificationType, title, body string) {
	n, err := h.DB.Insert(ctx, userID, typ, title, body)
	if err != nil {
		slog.Error("insert notification", "err", err)
		return
	}

	payload, _ := json.Marshal(n)
	if err := h.RT.Publish(ctx, userID, payload); err != nil {
		slog.Warn("publish live notification", "err", err)
	}

	if user, err := h.Auth.GetUser(userID); err == nil && user.Email != "" {
		if err := h.Email.Send(user.Email, title, body); err != nil {
			slog.Warn("send notification email", "err", err)
		}
	}
}

type subscriptionEvent struct {
	Type           string `json:"type"`
	SubscriptionID string `json:"subscription_id"`
	SubscriberID   string `json:"subscriber_id"`
	ChannelID      string `json:"channel_id"`
	CreatorID      string `json:"creator_id"`
}

func (h *Handler) HandleSubscriptionEvent(ctx context.Context, raw []byte) error {
	var e subscriptionEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil // malformed message isn't retryable -- log and move on, not an error to retry
	}
	if e.Type != "subscribed" || e.CreatorID == "" {
		return nil // "cancelled" isn't notified in this build; no creator_id means an older/malformed event
	}

	name := h.Auth.DisplayNameOr(e.SubscriberID)
	h.notify(ctx, e.CreatorID, model.TypeNewSubscriber,
		"New subscriber!",
		fmt.Sprintf("%s just subscribed to your channel.", name))
	return nil
}

type giftEvent struct {
	Type        string `json:"type"`
	GiftID      string `json:"gift_id"`
	SenderID    string `json:"sender_id"`
	RecipientID string `json:"recipient_id"`
	GiftType    string `json:"gift_type"`
	CoinCost    int64  `json:"coin_cost"`
}

func (h *Handler) HandleGiftEvent(ctx context.Context, raw []byte) error {
	var e giftEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "gift" {
		return nil
	}

	name := h.Auth.DisplayNameOr(e.SenderID)
	h.notify(ctx, e.RecipientID, model.TypeGiftReceived,
		"You received a gift!",
		fmt.Sprintf("%s sent you a %s (%d coins).", name, e.GiftType, e.CoinCost))
	return nil
}

type streamEvent struct {
	Type      string `json:"type"`
	StreamID  string `json:"stream_id"`
	ChannelID string `json:"channel_id"`
	CreatorID string `json:"creator_id"`
	Title     string `json:"title"`
}

// HandleStreamEvent fans a single "stream started" event out to every
// active subscriber -- the one handler here that produces N
// notifications from one Kafka message, not one.
func (h *Handler) HandleStreamEvent(ctx context.Context, raw []byte) error {
	var e streamEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil
	}
	if e.Type != "stream-started" {
		return nil
	}

	name := h.Auth.DisplayNameOr(e.CreatorID)
	subscriberIDs, err := h.Subscriptions.ListActiveSubscriberIDs(e.ChannelID)
	if err != nil {
		return err // retryable -- subscription-service might just be briefly unreachable
	}

	for _, subscriberID := range subscriberIDs {
		h.notify(ctx, subscriberID, model.TypeStreamStarted,
			fmt.Sprintf("%s is live!", name),
			fmt.Sprintf("%s just went live: %s", name, e.Title))
	}
	return nil
}
