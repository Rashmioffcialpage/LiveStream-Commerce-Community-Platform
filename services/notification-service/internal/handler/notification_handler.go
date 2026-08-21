package handler

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"

	"notification-service/internal/auth"
	"notification-service/internal/db"
)

func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	notifications, err := h.DB.ListByUser(r.Context(), claims.UserID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list notifications")
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id := r.PathValue("id")
	n, err := h.DB.MarkRead(r.Context(), id, claims.UserID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "notification not found, not yours, or already read")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not mark read")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

var upgrader = websocket.Upgrader{
	// see stream-service/chat-service's identical note: no cookie-based
	// auth crosses this socket, so there's no CSRF surface a stricter
	// origin check would be protecting here.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NotificationsWS pushes live notifications as they're produced. Recent
// history is delivered as the socket's first frames so a client that
// just reconnected doesn't need a separate REST call to catch up --
// same pattern as chat-service's initial "history" frame.
func (h *Handler) NotificationsWS(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if recent, err := h.DB.ListByUser(r.Context(), claims.UserID, 10); err == nil {
		for i := len(recent) - 1; i >= 0; i-- {
			_ = conn.WriteJSON(recent[i])
		}
	}

	ps := h.RT.Subscribe(r.Context(), claims.UserID)
	defer ps.Close()

	psCh := ps.Channel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		// drain client reads just to notice a closed connection -- this
		// socket is push-only, the client never sends anything meaningful
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case msg, ok := <-psCh:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
