package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"commerce-service/internal/auth"
	"commerce-service/internal/catalog"
	"commerce-service/internal/db"
	"commerce-service/internal/streamclient"
)

type sendGiftRequest struct {
	GiftType string `json:"gift_type"`
}

// SendGift implements Viewer -> Buy Coins (already done, via BuyCoins) ->
// Send Gift -> Creator: resolve the channel's creator, validate the gift
// type against the fixed catalog, then let DB.SendGift do the atomic
// debit-sender/credit-creator/record-gift transaction.
func (h *Handler) SendGift(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	slug := r.PathValue("slug")

	var req sendGiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	coinCost, ok := catalog.CoinCost(req.GiftType)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown gift_type")
		return
	}

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
		writeError(w, http.StatusBadRequest, "you cannot gift your own channel")
		return
	}

	gift, err := h.DB.SendGift(r.Context(), claims.UserID, channel.CreatorID, channel.ID, req.GiftType, coinCost)
	if errors.Is(err, db.ErrInsufficientBalance) {
		writeError(w, http.StatusPaymentRequired, "insufficient coin balance -- buy more coins first")
		return
	}
	if err != nil {
		slog.Error("send gift", "err", err)
		writeError(w, http.StatusInternalServerError, "could not send gift")
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"type": "gift", "gift_id": gift.ID, "sender_id": gift.SenderID,
		"recipient_id": gift.RecipientID, "channel_id": gift.ChannelID,
		"gift_type": gift.GiftType, "coin_cost": gift.CoinCost,
	})
	if err := h.Producer.Produce(r.Context(), []byte(gift.RecipientID), payload); err != nil {
		slog.Error("emit gift event", "err", err)
	}

	writeJSON(w, http.StatusCreated, gift)
}

func (h *Handler) GetCreatorBalance(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	balance, err := h.DB.GetCreatorBalance(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch balance")
		return
	}
	writeJSON(w, http.StatusOK, balance)
}

func (h *Handler) ListGiftsReceived(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	gifts, err := h.DB.ListGiftsReceived(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list gifts received")
		return
	}
	writeJSON(w, http.StatusOK, gifts)
}

func (h *Handler) ListGiftsSent(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	gifts, err := h.DB.ListGiftsSent(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list gifts sent")
		return
	}
	writeJSON(w, http.StatusOK, gifts)
}
