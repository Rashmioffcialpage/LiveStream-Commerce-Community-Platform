package handler

import (
	"encoding/json"
	"net/http"

	"subscription-service/internal/config"
	"subscription-service/internal/db"
	"subscription-service/internal/kafka"
	"subscription-service/internal/paymentclient"
	"subscription-service/internal/streamclient"
)

type Handler struct {
	DB       *db.DB
	Producer *kafka.Producer
	Payments *paymentclient.Client
	Streams  *streamclient.Client
	Cfg      config.Config
}

func New(database *db.DB, producer *kafka.Producer, payments *paymentclient.Client, streams *streamclient.Client, cfg config.Config) *Handler {
	return &Handler{DB: database, Producer: producer, Payments: payments, Streams: streams, Cfg: cfg}
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
