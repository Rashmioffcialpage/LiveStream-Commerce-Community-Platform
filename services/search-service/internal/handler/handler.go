package handler

import (
	"encoding/json"
	"net/http"

	"search-service/internal/authclient"
	"search-service/internal/config"
	"search-service/internal/search"
)

type Handler struct {
	Search *search.Client
	Auth   *authclient.Client
	Cfg    config.Config
}

func New(searchClient *search.Client, authC *authclient.Client, cfg config.Config) *Handler {
	return &Handler{Search: searchClient, Auth: authC, Cfg: cfg}
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
