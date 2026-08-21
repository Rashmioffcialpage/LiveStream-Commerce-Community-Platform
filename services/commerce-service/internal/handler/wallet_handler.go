package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"commerce-service/internal/auth"
	"commerce-service/internal/paymentclient"
)

func (h *Handler) GetWallet(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	wallet, err := h.DB.GetWallet(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch wallet")
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

type buyCoinsRequest struct {
	Coins int64 `json:"coins"`
}

// BuyCoins is the "Viewer -> Buy Coins" half of the gifting flow -- charge
// via payment-service, then credit the wallet only once that charge
// actually succeeded (same charge-then-write ordering as
// subscription-service's Subscribe).
func (h *Handler) BuyCoins(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req buyCoinsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Coins <= 0 {
		writeError(w, http.StatusBadRequest, "coins must be a positive number")
		return
	}

	amountCents := int(req.Coins) * h.Cfg.CentsPerCoin
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}

	charge, err := h.Payments.Charge(bearerToken(r), amountCents, "usd", fmt.Sprintf("Buy %d coins", req.Coins), idempotencyKey)
	if errors.Is(err, paymentclient.ErrDeclined) {
		writeError(w, http.StatusPaymentRequired, "payment declined")
		return
	}
	if err != nil {
		slog.Error("charge for coins", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach payment-service")
		return
	}

	wallet, err := h.DB.BuyCoins(r.Context(), claims.UserID, req.Coins, amountCents, charge.ID)
	if err != nil {
		slog.Error("credit wallet", "err", err)
		writeError(w, http.StatusInternalServerError, "charged but could not credit wallet")
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}
