package handler

import (
	"encoding/json"
	"net/http"

	"chat-service/internal/config"
	"chat-service/internal/db"
	"chat-service/internal/kafka"
	"chat-service/internal/realtime"
	"chat-service/internal/streamclient"
)

type Handler struct {
	DB       *db.DB
	RT       *realtime.Client
	Producer *kafka.Producer
	Streams  *streamclient.Client
	Cfg      config.Config
}

func New(database *db.DB, rt *realtime.Client, producer *kafka.Producer, streams *streamclient.Client, cfg config.Config) *Handler {
	return &Handler{DB: database, RT: rt, Producer: producer, Streams: streams, Cfg: cfg}
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
