package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"subscription-service/internal/auth"
	"subscription-service/internal/db"
	"subscription-service/internal/paymentclient"
	"subscription-service/internal/streamclient"
)

const subscriptionPeriod = 30 * 24 * time.Hour

// Subscribe implements the spec's Viewer -> Subscribe -> Payment Service
// -> Subscription DB -> Kafka Event chain: charge first (payment-service
// is the source of truth for "did money move"), then write the
// subscription row only once the charge actually succeeded, then emit the
// event. Each step only happens if the previous one succeeded -- there's
// no subscription row for a declined charge, and no Kafka event for a
// subscription that was never created.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	slug := r.PathValue("slug")

	channel, err := h.Streams.GetChannelBySlug(slug)
	if errors.Is(err, streamclient.ErrNotFound) {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach stream-service")
		return
	}
	if channel.CreatorID == claims.UserID {
		writeError(w, http.StatusBadRequest, "you cannot subscribe to your own channel")
		return
	}

	already, err := h.DB.HasActiveSubscription(r.Context(), claims.UserID, channel.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not check existing subscription")
		return
	}
	if already {
		writeError(w, http.StatusConflict, "already subscribed to this channel")
		return
	}

	token := bearerToken(r)
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		// no client-supplied key means a network-level retry of this exact
		// request could charge twice -- accepted here since this is a
		// demo/portfolio payment simulation, not a real card network;
		// flagged so it's not mistaken for an oversight.
		idempotencyKey = uuid.NewString()
	}

	charge, err := h.Payments.Charge(token, h.Cfg.SubscriptionPriceCents, "usd", fmt.Sprintf("Subscription: %s", channel.Name), idempotencyKey)
	if errors.Is(err, paymentclient.ErrDeclined) {
		writeError(w, http.StatusPaymentRequired, "payment declined")
		return
	}
	if err != nil {
		slog.Error("charge subscriber", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach payment-service")
		return
	}

	sub, err := h.DB.CreateSubscription(r.Context(), claims.UserID, channel.ID, charge.ID, time.Now().Add(subscriptionPeriod))
	if errors.Is(err, db.ErrAlreadySubscribed) {
		// lost the race against a concurrent subscribe -- the charge above
		// already succeeded and is a legitimate, non-refunded payment; this
		// is a demo, so it's surfaced as an error rather than building a
		// refund path for a case this unlikely in practice.
		writeError(w, http.StatusConflict, "already subscribed to this channel (a concurrent request beat this one)")
		return
	}
	if err != nil {
		slog.Error("create subscription", "err", err)
		writeError(w, http.StatusInternalServerError, "charged but could not record subscription")
		return
	}

	h.emitEvent(r.Context(), "subscribed", sub.ID, sub.SubscriberID, sub.ChannelID)
	writeJSON(w, http.StatusCreated, sub)
}

func (h *Handler) ListSubscribers(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	slug := r.PathValue("slug")

	channel, err := h.Streams.GetChannelBySlug(slug)
	if errors.Is(err, streamclient.ErrNotFound) {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach stream-service")
		return
	}
	if channel.CreatorID != claims.UserID {
		writeError(w, http.StatusForbidden, "you do not own this channel")
		return
	}

	subs, err := h.DB.ListActiveSubscribersByChannel(r.Context(), channel.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list subscribers")
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) MySubscriptions(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	subs, err := h.DB.ListSubscriptionsBySubscriber(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list subscriptions")
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id := r.PathValue("id")

	existing, err := h.DB.GetSubscription(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch subscription")
		return
	}
	if existing.SubscriberID != claims.UserID {
		writeError(w, http.StatusForbidden, "not your subscription")
		return
	}

	cancelled, err := h.DB.CancelSubscription(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusConflict, "subscription is not active")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not cancel subscription")
		return
	}

	h.emitEvent(r.Context(), "cancelled", cancelled.ID, cancelled.SubscriberID, cancelled.ChannelID)
	writeJSON(w, http.StatusOK, cancelled)
}

func (h *Handler) emitEvent(ctx context.Context, eventType, subID, subscriberID, channelID string) {
	payload, _ := json.Marshal(map[string]string{
		"type": eventType, "subscription_id": subID, "subscriber_id": subscriberID, "channel_id": channelID,
	})
	if err := h.Producer.Produce(ctx, []byte(channelID), payload); err != nil {
		slog.Error("emit subscription event", "type", eventType, "err", err)
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) {
		return h[len(prefix):]
	}
	return ""
}
