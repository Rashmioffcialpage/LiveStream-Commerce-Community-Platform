package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"chat-service/internal/auth"
	"chat-service/internal/model"
	"chat-service/internal/moderation"
	"chat-service/internal/streamclient"
)

var upgrader = websocket.Upgrader{
	// see stream-service/internal/handler/signal_handler.go for the same
	// note: no cookie-based auth crosses this socket, so there's no CSRF
	// surface to protect with a stricter origin check in this deployment.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type outbound struct {
	Type        string    `json:"type"`
	ID          string    `json:"id,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	Body        string    `json:"body,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	Messages    []model.ChatMessage `json:"messages,omitempty"`
}

type inbound struct {
	Type model.MessageType `json:"type"`
	Body string            `json:"body"`
}

// ChatWS is the live chat WebSocket: RequireAuth (any role) has already
// run, so claims are in the request context. Every accepted message is
// broadcast immediately via Redis Pub/Sub (low latency) and produced to
// Kafka for durable history (see internal/kafka) -- rejected messages
// (muted, rate-limited, banned content) never reach either.
func (h *Handler) ChatWS(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	streamID := r.PathValue("id")

	stream, err := h.Streams.GetStream(streamID)
	if err == streamclient.ErrNotFound {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach stream-service")
		return
	}
	if stream.Status != "live" {
		writeError(w, http.StatusConflict, "stream is not live")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("chat ws: upgrade", "err", err)
		return
	}
	defer conn.Close()

	send := make(chan []byte, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for payload := range send {
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}()
	defer close(send)

	// initial backfill so a client doesn't have to make a separate REST
	// call before it can render anything
	history, err := h.DB.History(r.Context(), streamID, 50)
	if err == nil {
		reverseChatMessages(history)
		if payload, err := json.Marshal(outbound{Type: "history", Messages: history}); err == nil {
			send <- payload
		}
	}

	ps := h.RT.Subscribe(r.Context(), streamID)
	defer ps.Close()
	psCh := ps.Channel()
	go func() {
		for msg := range psCh {
			select {
			case send <- []byte(msg.Payload):
			case <-done:
				return
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		h.handleInbound(r.Context(), claims, stream.ChannelID, streamID, raw, send)
	}
}

func (h *Handler) handleInbound(ctx context.Context, claims *auth.Claims, channelID, streamID string, raw []byte, send chan<- []byte) {
	var in inbound
	if err := json.Unmarshal(raw, &in); err != nil {
		reply(send, outbound{Type: "error", Reason: "malformed message"})
		return
	}

	if in.Type != model.TypeMessage && in.Type != model.TypeReaction {
		reply(send, outbound{Type: "error", Reason: "type must be \"message\" or \"reaction\""})
		return
	}
	if in.Type == model.TypeReaction {
		if !moderation.IsAllowedReaction(in.Body) {
			reply(send, outbound{Type: "error", Reason: "unsupported reaction"})
			return
		}
	} else {
		if in.Body == "" || len(in.Body) > moderation.MaxMessageLength {
			reply(send, outbound{Type: "error", Reason: "message must be 1-500 characters"})
			return
		}
		if moderation.ContainsBannedWord(in.Body) {
			reply(send, outbound{Type: "error", Reason: "message rejected by content filter"})
			return
		}
	}

	muted, err := h.RT.IsMuted(ctx, channelID, claims.UserID)
	if err != nil {
		slog.Warn("chat: mute check failed", "err", err)
	}
	if muted {
		reply(send, outbound{Type: "error", Reason: "you are muted in this channel"})
		return
	}

	allowed, err := h.RT.Allow(ctx, streamID, claims.UserID)
	if err != nil {
		slog.Warn("chat: rate limit check failed", "err", err)
	}
	if !allowed {
		reply(send, outbound{Type: "error", Reason: "rate limited -- slow down"})
		return
	}

	msg := model.ChatMessage{
		ID:          uuid.NewString(),
		StreamID:    streamID,
		UserID:      claims.UserID,
		DisplayName: claims.DisplayName,
		Type:        in.Type,
		Body:        in.Body,
		CreatedAt:   time.Now().UTC(),
	}
	payload, err := json.Marshal(outbound{
		Type: string(msg.Type), ID: msg.ID, UserID: msg.UserID,
		DisplayName: msg.DisplayName, Body: msg.Body, CreatedAt: msg.CreatedAt,
	})
	if err != nil {
		return
	}

	// broadcast first (latency path), persist second (durability path) --
	// a Kafka blip delays history, it never delays what's on screen live
	if err := h.RT.Publish(ctx, streamID, payload); err != nil {
		slog.Error("chat: publish failed", "err", err)
	}
	go func() {
		produceCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.Producer.Produce(produceCtx, []byte(streamID), mustMarshal(msg)); err != nil {
			slog.Error("chat: kafka produce failed", "err", err)
		}
	}()
}

func reply(send chan<- []byte, o outbound) {
	if payload, err := json.Marshal(o); err == nil {
		select {
		case send <- payload:
		default:
		}
	}
}

func mustMarshal(m model.ChatMessage) []byte {
	b, _ := json.Marshal(m)
	return b
}

func reverseChatMessages(m []model.ChatMessage) {
	for i, j := 0, len(m)-1; i < j; i, j = i+1, j-1 {
		m[i], m[j] = m[j], m[i]
	}
}
