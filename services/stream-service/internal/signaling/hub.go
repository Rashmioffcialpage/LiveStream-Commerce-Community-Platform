// Package signaling implements the WebRTC signaling relay: it never touches
// media. Broadcaster and viewer browsers negotiate their own RTCPeerConnection
// (SDP offer/answer, ICE candidates) and exchange audio/video directly (or
// via TURN); this package's only job is delivering those SDP/ICE messages
// between the right two parties, since a browser has no way to reach another
// browser without a rendezvous point. One room per live stream, one
// broadcaster, N viewers, star topology -- production scale-out beyond a
// handful of concurrent viewers per stream needs an SFU (e.g. mediasoup,
// LiveKit) in front of this relay; that's explicitly the "Twitch video
// infra" this project isn't rebuilding from scratch.
package signaling

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

type MessageType string

const (
	TypeOffer        MessageType = "offer"
	TypeAnswer       MessageType = "answer"
	TypeICECandidate MessageType = "ice-candidate"
	TypeViewerJoined MessageType = "viewer-joined"
	TypeViewerLeft   MessageType = "viewer-left"
	TypeBroadcasterGone MessageType = "broadcaster-left"
	TypeViewerCount  MessageType = "viewer-count"
	TypeError        MessageType = "error"
)

type Message struct {
	Type    MessageType     `json:"type"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

var ErrRoomHasBroadcaster = errors.New("stream already has an active broadcaster connection")

type Client struct {
	ConnID string
	IsBroadcaster bool
	conn   *websocket.Conn
	send   chan Message
}

func newClient(connID string, isBroadcaster bool, conn *websocket.Conn) *Client {
	return &Client{ConnID: connID, IsBroadcaster: isBroadcaster, conn: conn, send: make(chan Message, 16)}
}

// writePump is the only goroutine that ever calls conn.WriteJSON -- gorilla
// websocket connections aren't safe for concurrent writes, so every relay
// hop goes through this channel instead of writing directly.
func (c *Client) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteJSON(msg); err != nil {
			return
		}
	}
}

type room struct {
	mu          sync.Mutex
	broadcaster *Client
	viewers     map[string]*Client
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*room
}

func NewHub() *Hub {
	return &Hub{rooms: map[string]*room{}}
}

func (h *Hub) getOrCreateRoom(streamID string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[streamID]
	if !ok {
		r = &room{viewers: map[string]*Client{}}
		h.rooms[streamID] = r
	}
	return r
}

// JoinBroadcaster registers the stream owner's signaling connection. Only
// one broadcaster connection per stream is allowed -- a second attempt
// (e.g. the creator opening a second tab) is rejected rather than silently
// replacing the first, so viewers don't get orphaned mid-negotiation.
func (h *Hub) JoinBroadcaster(streamID string, conn *websocket.Conn) (*Client, error) {
	r := h.getOrCreateRoom(streamID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.broadcaster != nil {
		return nil, ErrRoomHasBroadcaster
	}
	c := newClient("broadcaster", true, conn)
	r.broadcaster = c
	go c.writePump()
	return c, nil
}

func (h *Hub) JoinViewer(streamID, connID string, conn *websocket.Conn) *Client {
	r := h.getOrCreateRoom(streamID)
	c := newClient(connID, false, conn)
	go c.writePump()

	r.mu.Lock()
	r.viewers[connID] = c
	bc := r.broadcaster
	n := len(r.viewers)
	r.mu.Unlock()

	if bc != nil {
		bc.send <- Message{Type: TypeViewerJoined, From: connID}
	}
	h.broadcastViewerCount(streamID, n)
	return c
}

// Route delivers one signaling message from sender to its intended
// recipient(s) within the same room:
//   - broadcaster -> viewer: requires "to" (which viewer's offer/candidate)
//   - viewer -> broadcaster: "from" is set by the relay, not trusted from
//     the client, so a viewer can't spoof another viewer's connID
func (h *Hub) Route(streamID string, sender *Client, msg Message) {
	h.mu.Lock()
	r, ok := h.rooms[streamID]
	h.mu.Unlock()
	if !ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if sender.IsBroadcaster {
		target, ok := r.viewers[msg.To]
		if !ok {
			return
		}
		msg.From = "broadcaster"
		select {
		case target.send <- msg:
		default:
		}
		return
	}

	if r.broadcaster == nil {
		return
	}
	msg.From = sender.ConnID
	msg.To = ""
	select {
	case r.broadcaster.send <- msg:
	default:
	}
}

// Leave removes a client from its room and notifies the other side --
// viewers learn the stream ended, the broadcaster learns a viewer's
// RTCPeerConnection can be torn down.
func (h *Hub) Leave(streamID string, c *Client) {
	h.mu.Lock()
	r, ok := h.rooms[streamID]
	h.mu.Unlock()
	if !ok {
		return
	}

	r.mu.Lock()
	if c.IsBroadcaster {
		r.broadcaster = nil
	} else {
		delete(r.viewers, c.ConnID)
	}
	bc := r.broadcaster
	viewers := make([]*Client, 0, len(r.viewers))
	for _, v := range r.viewers {
		viewers = append(viewers, v)
	}
	n := len(r.viewers)
	r.mu.Unlock()

	close(c.send)

	if c.IsBroadcaster {
		for _, v := range viewers {
			select {
			case v.send <- Message{Type: TypeBroadcasterGone}:
			default:
			}
		}
	} else if bc != nil {
		select {
		case bc.send <- Message{Type: TypeViewerLeft, From: c.ConnID}:
		default:
		}
	}
	h.broadcastViewerCount(streamID, n)
}

func (h *Hub) broadcastViewerCount(streamID string, n int) {
	h.mu.Lock()
	r, ok := h.rooms[streamID]
	h.mu.Unlock()
	if !ok {
		return
	}
	payload, _ := json.Marshal(map[string]int{"count": n})
	msg := Message{Type: TypeViewerCount, Payload: payload}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.broadcaster != nil {
		select {
		case r.broadcaster.send <- msg:
		default:
		}
	}
	for _, v := range r.viewers {
		select {
		case v.send <- msg:
		default:
			slog.Warn("signaling: dropped viewer-count message, send buffer full", "streamID", streamID)
		}
	}
}
