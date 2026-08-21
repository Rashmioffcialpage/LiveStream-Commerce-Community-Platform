package handler

import (
	"encoding/json"
	"net/http"

	"stream-service/internal/config"
	"stream-service/internal/db"
	"stream-service/internal/kafka"
	"stream-service/internal/realtime"
	"stream-service/internal/signaling"
	"stream-service/internal/storage"
)

type Handler struct {
	DB       *db.DB
	Presence *realtime.Presence
	Hub      *signaling.Hub
	Storage  *storage.Client
	Producer *kafka.Producer
	Cfg      config.Config
}

func New(database *db.DB, presence *realtime.Presence, hub *signaling.Hub, store *storage.Client, producer *kafka.Producer, cfg config.Config) *Handler {
	return &Handler{DB: database, Presence: presence, Hub: hub, Storage: store, Producer: producer, Cfg: cfg}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
