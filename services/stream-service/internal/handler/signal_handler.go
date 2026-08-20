package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"stream-service/internal/auth"
	"stream-service/internal/db"
	"stream-service/internal/signaling"
)

var upgrader = websocket.Upgrader{
	// Any origin is accepted -- this is a demo/portfolio deployment behind
	// no cookie-based auth (the signaling socket itself carries its own
	// bearer token in the query string for the broadcaster leg), so there's
	// no cross-origin credential to protect against CSRF here. A production
	// deployment would restrict this to the known frontend origin(s).
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Signal upgrades to a WebSocket and relays WebRTC SDP/ICE messages between
// exactly one broadcaster and any number of viewers for a given stream.
// See internal/signaling for the relay logic; this handler only owns
// authorization, the room lifecycle (join/leave), and presence counting.
func (h *Handler) Signal(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("id")
	role := r.URL.Query().Get("role")

	stream, err := h.DB.GetStream(r.Context(), streamID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch stream")
		return
	}
	if stream.Status != "live" {
		writeError(w, http.StatusConflict, "stream is not live -- call POST /streams/{id}/go-live first")
		return
	}

	switch role {
	case "broadcaster":
		h.signalBroadcaster(w, r, stream.ChannelID, streamID)
	case "viewer":
		h.signalViewer(w, r, streamID)
	default:
		writeError(w, http.StatusBadRequest, `role query param must be "broadcaster" or "viewer"`)
	}
}

func (h *Handler) signalBroadcaster(w http.ResponseWriter, r *http.Request, channelID, streamID string) {
	// the WebSocket upgrade request can't carry an Authorization header
	// from a browser client, so the broadcaster leg takes the bearer token
	// as a query param instead.
	token := r.URL.Query().Get("token")
	claims, err := auth.ParseAccessToken(h.Cfg.JWTSecret, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing or invalid token query param")
		return
	}
	channel, err := h.DB.GetChannelByID(r.Context(), channelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch channel")
		return
	}
	if channel.CreatorID != claims.UserID {
		writeError(w, http.StatusForbidden, "you do not own this stream's channel")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("signal: broadcaster upgrade", "err", err)
		return
	}

	client, err := h.Hub.JoinBroadcaster(streamID, conn)
	if err != nil {
		_ = conn.WriteJSON(signaling.Message{Type: signaling.TypeError, Payload: mustJSON(err.Error())})
		conn.Close()
		return
	}
	h.readLoop(streamID, client, conn)
}

func (h *Handler) signalViewer(w http.ResponseWriter, r *http.Request, streamID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("signal: viewer upgrade", "err", err)
		return
	}

	connID := uuid.NewString()
	client := h.Hub.JoinViewer(streamID, connID, conn)
	if err := h.Presence.Join(r.Context(), streamID, connID); err != nil {
		slog.Warn("signal: presence join failed", "err", err)
	}
	h.readLoop(streamID, client, conn)
	if err := h.Presence.Leave(r.Context(), streamID, connID); err != nil {
		slog.Warn("signal: presence leave failed", "err", err)
	}
}

// readLoop blocks until the connection closes, dispatching every inbound
// message to the hub. Runs on the goroutine the HTTP handler was called on;
// Client.writePump (started when the client joined the hub) is the only
// other goroutine touching this connection, and it only ever writes.
func (h *Handler) readLoop(streamID string, client *signaling.Client, conn *websocket.Conn) {
	defer func() {
		h.Hub.Leave(streamID, client)
		conn.Close()
	}()
	for {
		var msg signaling.Message
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		h.Hub.Route(streamID, client, msg)
	}
}

func mustJSON(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
