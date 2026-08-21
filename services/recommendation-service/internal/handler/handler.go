package handler

import (
	"encoding/json"
	"net/http"

	"recommendation-service/internal/config"
	"recommendation-service/internal/features"
	"recommendation-service/internal/streamclient"
)

type Handler struct {
	Features *features.Store
	Streams  *streamclient.Client
	Cfg      config.Config
}

func New(store *features.Store, streams *streamclient.Client, cfg config.Config) *Handler {
	return &Handler{Features: store, Streams: streams, Cfg: cfg}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
