package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"stream-service/internal/auth"
	"stream-service/internal/db"
	"stream-service/internal/kafka"
)

type createStreamRequest struct {
	Title            string    `json:"title"`
	Tags             []string  `json:"tags"`
	ScheduledStartAt time.Time `json:"scheduled_start_at"`
}

func (h *Handler) CreateStream(w http.ResponseWriter, r *http.Request) {
	channelID, ok := h.requireChannelOwnership(w, r)
	if !ok {
		return
	}

	var req createStreamRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.ScheduledStartAt.IsZero() {
		req.ScheduledStartAt = time.Now()
	}

	stream, err := h.DB.CreateStream(r.Context(), channelID, req.Title, req.Tags, req.ScheduledStartAt)
	if err != nil {
		slog.Error("create stream", "err", err)
		writeError(w, http.StatusInternalServerError, "could not schedule stream")
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"type": "stream-created", "stream_id": stream.ID, "channel_id": stream.ChannelID,
		"title": stream.Title, "tags": stream.Tags,
	})
	if err := h.Producer.Produce(r.Context(), kafka.TopicStream, []byte(stream.ChannelID), payload); err != nil {
		slog.Error("emit stream-created event", "err", err)
	}

	writeJSON(w, http.StatusCreated, stream)
}

func (h *Handler) ListChannelStreams(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	channel, err := h.DB.GetChannelBySlug(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch channel")
		return
	}

	streams, err := h.DB.ListStreamsByChannel(r.Context(), channel.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list streams")
		return
	}
	for i := range streams {
		if streams[i].Status == "live" {
			n, _ := h.Presence.Count(r.Context(), streams[i].ID)
			streams[i].ViewerCount = n
		}
	}
	writeJSON(w, http.StatusOK, streams)
}

// GoLive and EndStream re-derive channel ownership from the stream's own
// channel_id rather than trusting a slug in the URL, since these routes
// are addressed by stream ID, not nested under /channels/{slug}.
func (h *Handler) requireStreamOwnership(w http.ResponseWriter, r *http.Request, streamID string) bool {
	stream, err := h.DB.GetStream(r.Context(), streamID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream not found")
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch stream")
		return false
	}
	channel, err := h.DB.GetChannelByID(r.Context(), stream.ChannelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch channel")
		return false
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if channel.CreatorID != claims.UserID {
		writeError(w, http.StatusForbidden, "you do not own this stream's channel")
		return false
	}
	return true
}

func (h *Handler) GoLive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.requireStreamOwnership(w, r, id) {
		return
	}
	stream, err := h.DB.GoLive(r.Context(), id)
	if errors.Is(err, db.ErrAlreadyLive) {
		writeError(w, http.StatusConflict, "this channel already has a live stream")
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusConflict, "stream is not in scheduled status")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not go live")
		return
	}

	if channel, chErr := h.DB.GetChannelByID(r.Context(), stream.ChannelID); chErr == nil {
		payload, _ := json.Marshal(map[string]string{
			"type": "stream-started", "stream_id": stream.ID,
			"channel_id": stream.ChannelID, "creator_id": channel.CreatorID, "title": stream.Title,
		})
		if err := h.Producer.Produce(r.Context(), kafka.TopicStream, []byte(stream.ChannelID), payload); err != nil {
			slog.Error("emit stream-started event", "err", err)
		}
	}

	writeJSON(w, http.StatusOK, stream)
}

func (h *Handler) EndStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.requireStreamOwnership(w, r, id) {
		return
	}
	stream, err := h.DB.EndStream(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusConflict, "stream is not currently live")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not end stream")
		return
	}
	writeJSON(w, http.StatusOK, stream)
}

func (h *Handler) GetStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stream, err := h.DB.GetStream(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch stream")
		return
	}
	if stream.Status == "live" {
		n, _ := h.Presence.Count(r.Context(), stream.ID)
		stream.ViewerCount = n
	}
	writeJSON(w, http.StatusOK, stream)
}
