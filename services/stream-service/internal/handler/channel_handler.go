package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"stream-service/internal/auth"
	"stream-service/internal/db"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,48}[a-z0-9])?$`)

type createChannelRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// CreateChannel requires a creator-role JWT (enforced by RequireRole in
// main.go's route wiring) -- viewers can't create channels.
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req createChannelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if !slugPattern.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must be 3-50 lowercase alphanumeric characters or hyphens")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	channel, err := h.DB.CreateChannel(r.Context(), claims.UserID, req.Slug, req.Name, req.Description, req.Category)
	if errors.Is(err, db.ErrConflict) {
		writeError(w, http.StatusConflict, "that channel slug is already taken")
		return
	}
	if err != nil {
		slog.Error("create channel", "err", err)
		writeError(w, http.StatusInternalServerError, "could not create channel")
		return
	}
	writeJSON(w, http.StatusCreated, channel)
}

func (h *Handler) GetChannel(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, channel)
}

func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	channels, err := h.DB.ListChannels(r.Context(), category, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list channels")
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

// requireChannelOwnership loads the channel and checks the caller's JWT
// user ID against creator_id. Every write route under /channels/{slug}/...
// goes through this so ownership is enforced in one place instead of
// re-checked ad hoc per handler.
func (h *Handler) requireChannelOwnership(w http.ResponseWriter, r *http.Request) (channelID string, ok bool) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	slug := r.PathValue("slug")

	channel, err := h.DB.GetChannelBySlug(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "channel not found")
		return "", false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch channel")
		return "", false
	}
	if channel.CreatorID != claims.UserID {
		writeError(w, http.StatusForbidden, "you do not own this channel")
		return "", false
	}
	return channel.ID, true
}
