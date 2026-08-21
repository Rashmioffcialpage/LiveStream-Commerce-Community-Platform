package handler

import (
	"log/slog"
	"net/http"
	"sort"

	"recommendation-service/internal/auth"
	"recommendation-service/internal/streamclient"
)

type feedItem struct {
	streamclient.Channel
	Score float64 `json:"score"`
}

// GetFeed is the "-> Personalized Home Feed" step: rank every channel by
// the caller's own category affinity (from RecordEvent, via
// TopCategories), highest first. A user recommendation-service has never
// seen has no affinity data at all -- every channel scores 0 and the
// list comes back in stream-service's default (most-recent-first) order,
// which is the correct cold-start behavior, not a special case to code
// around.
func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var cached []feedItem
	if hit, err := h.Features.CachedFeed(r.Context(), claims.UserID, &cached); err == nil && hit {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	channels, err := h.Streams.ListChannels()
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach stream-service")
		return
	}

	topCategories, err := h.Features.TopCategories(r.Context(), claims.UserID, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read affinity scores")
		return
	}
	affinityByCategory := make(map[string]float64, len(topCategories))
	for _, c := range topCategories {
		affinityByCategory[c.Category] = c.Score
	}

	feed := make([]feedItem, len(channels))
	for i, ch := range channels {
		feed[i] = feedItem{Channel: ch, Score: affinityByCategory[ch.Category]}
	}
	// stable sort: channels with equal score (including the all-zero cold-
	// start case) keep stream-service's original most-recent-first order
	sort.SliceStable(feed, func(i, j int) bool { return feed[i].Score > feed[j].Score })

	// cache is a performance optimization, not correctness -- a failure
	// here still serves the freshly computed feed, just without the
	// speedup on the next request
	if err := h.Features.CacheFeed(r.Context(), claims.UserID, feed); err != nil {
		slog.Warn("cache feed", "err", err)
	}
	writeJSON(w, http.StatusOK, feed)
}
