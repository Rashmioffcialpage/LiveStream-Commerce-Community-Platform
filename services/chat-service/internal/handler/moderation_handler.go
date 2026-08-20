package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chat-service/internal/auth"
	"chat-service/internal/db"
)

const defaultMuteDuration = 10 * time.Minute

type muteRequest struct {
	UserID          string `json:"user_id"`
	DurationSeconds int    `json:"duration_seconds"`
}

// Mute and Unmute are scoped by stream ID in the URL but authorized by
// channel ownership -- a mute applies channel-wide (consistent with a
// creator wanting to moderate their whole community, not just one
// broadcast), resolved via stream -> channel -> creator_id.
func (h *Handler) Mute(w http.ResponseWriter, r *http.Request) {
	channelID, ok := h.requireStreamChannelOwnership(w, r)
	if !ok {
		return
	}
	var req muteRequest
	if !decodeMuteRequest(w, r, &req) {
		return
	}
	duration := defaultMuteDuration
	if req.DurationSeconds > 0 {
		duration = time.Duration(req.DurationSeconds) * time.Second
	}
	if err := h.RT.Mute(r.Context(), channelID, req.UserID, duration); err != nil {
		writeError(w, http.StatusInternalServerError, "could not mute user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"muted": req.UserID, "duration_seconds": int(duration.Seconds())})
}

func (h *Handler) Unmute(w http.ResponseWriter, r *http.Request) {
	channelID, ok := h.requireStreamChannelOwnership(w, r)
	if !ok {
		return
	}
	var req muteRequest
	if !decodeMuteRequest(w, r, &req) {
		return
	}
	if err := h.RT.Unmute(r.Context(), channelID, req.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not unmute user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"unmuted": req.UserID})
}

// DeleteMessage soft-deletes and broadcasts a message-deleted event so
// connected clients pull it from their view live, not just future loads.
func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id := r.PathValue("id")

	existing, err := h.DB.GetMessageByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch message")
		return
	}

	owns, err := h.Streams.OwnsStream(existing.StreamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not verify ownership")
		return
	}
	if !owns {
		writeError(w, http.StatusForbidden, "you do not own this stream's channel")
		return
	}

	deleted, err := h.DB.SoftDelete(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusConflict, "message already deleted")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete message")
		return
	}

	payload, _ := json.Marshal(outbound{Type: "message-deleted", ID: deleted.ID})
	if err := h.RT.Publish(r.Context(), deleted.StreamID, payload); err != nil {
		slog.Warn("chat: broadcast delete failed", "err", err)
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *Handler) requireStreamChannelOwnership(w http.ResponseWriter, r *http.Request) (channelID string, ok bool) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	streamID := r.PathValue("id")

	stream, err := h.Streams.GetStream(streamID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch stream")
		return "", false
	}
	channel, err := h.Streams.GetChannel(stream.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch channel")
		return "", false
	}
	if channel.CreatorID != claims.UserID {
		writeError(w, http.StatusForbidden, "you do not own this stream's channel")
		return "", false
	}
	return channel.ID, true
}

func decodeMuteRequest(w http.ResponseWriter, r *http.Request, dst *muteRequest) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil || dst.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return false
	}
	return true
}
