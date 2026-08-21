package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"payment-service/internal/auth"
	"payment-service/internal/config"
	"payment-service/internal/db"
	"payment-service/internal/model"
)

type Handler struct {
	DB  *db.DB
	Cfg config.Config
}

func New(database *db.DB, cfg config.Config) *Handler {
	return &Handler{DB: database, Cfg: cfg}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type chargeRequest struct {
	AmountCents    int    `json:"amount_cents"`
	Currency       string `json:"currency"`
	Description    string `json:"description"`
	IdempotencyKey string `json:"idempotency_key"`
}

// declinedAmountCents is a fixed "this card gets declined" sentinel, the
// same convention Stripe's own test-card numbers use -- lets a caller
// (subscription-service, gifting) deterministically exercise the failure
// path in tests/demos without needing a real payment gateway's test mode.
const declinedAmountCents = 66600

// Charge simulates processing a payment: there's no real card network
// here, no real money moves. What's real is the contract a service like
// subscription-service integrates against -- idempotency, a durable
// charges record, and a success/decline outcome -- so swapping this for
// a real Stripe integration later changes this handler's internals, not
// anything about how callers use it.
func (h *Handler) Charge(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req chargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	if req.AmountCents <= 0 {
		writeError(w, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	if req.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}
	if req.Currency == "" {
		req.Currency = "usd"
	}

	if existing, err := h.DB.GetChargeByIdempotencyKey(r.Context(), req.IdempotencyKey); err == nil {
		writeChargeResponse(w, existing)
		return
	} else if !errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "could not check idempotency")
		return
	}

	status := model.StatusSucceeded
	if req.AmountCents == declinedAmountCents {
		status = model.StatusFailed
	}

	charge, err := h.DB.InsertCharge(r.Context(), claims.UserID, req.AmountCents, req.Currency, req.Description, status, req.IdempotencyKey)
	if err != nil {
		slog.Error("insert charge", "err", err)
		writeError(w, http.StatusInternalServerError, "could not process charge")
		return
	}
	writeChargeResponse(w, charge)
}

func writeChargeResponse(w http.ResponseWriter, c *model.Charge) {
	status := http.StatusCreated
	if c.Status == model.StatusFailed {
		status = http.StatusPaymentRequired
	}
	writeJSON(w, status, c)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
