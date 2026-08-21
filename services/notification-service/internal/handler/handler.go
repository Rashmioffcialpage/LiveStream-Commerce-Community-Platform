package handler

import (
	"encoding/json"
	"net/http"

	"notification-service/internal/authclient"
	"notification-service/internal/config"
	"notification-service/internal/db"
	"notification-service/internal/email"
	"notification-service/internal/realtime"
	"notification-service/internal/subscriptionclient"
)

type Handler struct {
	DB            *db.DB
	RT            *realtime.Client
	Auth          *authclient.Client
	Subscriptions *subscriptionclient.Client
	Email         email.Sender
	Cfg           config.Config
}

func New(database *db.DB, rt *realtime.Client, authC *authclient.Client, subsC *subscriptionclient.Client, sender email.Sender, cfg config.Config) *Handler {
	return &Handler{DB: database, RT: rt, Auth: authC, Subscriptions: subsC, Email: sender, Cfg: cfg}
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
