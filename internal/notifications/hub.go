package notifications

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Event represents a payload delivered to notification subscribers.
type Event struct {
	Event          string      `json:"event"`
	Notification   interface{} `json:"notification,omitempty"`
	NotificationID string      `json:"notification_id,omitempty"`
}

type client struct {
	conn *websocket.Conn
	send chan Event
}

// Hub fan-outs notification events to connected subscribers.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{}
}

// NewHub constructs a notification hub instance.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*client]struct{}),
	}
}

// Serve upgrades the HTTP connection to a WebSocket and registers the user subscriber.
func (h *Hub) Serve(userID string, w http.ResponseWriter, r *http.Request) {
	server := websocket.Server{
		Handshake: func(config *websocket.Config, req *http.Request) error {
			config.Protocol = append(config.Protocol, "json")
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			_ = conn.SetDeadline(time.Now().Add(5 * time.Minute))
			cl := &client{
				conn: conn,
				send: make(chan Event, 16),
			}

			h.addClient(userID, cl)
			defer h.removeClient(userID, cl)

			go h.writeLoop(cl)
			h.readLoop(cl)
		},
	}

	server.ServeHTTP(w, r)
}

// Broadcast delivers an event to all subscribers for the provided user ID.
func (h *Hub) Broadcast(userID string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients[userID] {
		select {
		case client.send <- event:
		default:
			// Drop if buffer full to avoid blocking all clients.
		}
	}
}

// BroadcastMany delivers an event to each supplied user ID.
func (h *Hub) BroadcastMany(userIDs []string, event Event) {
	for _, userID := range userIDs {
		h.Broadcast(userID, event)
	}
}

func (h *Hub) addClient(userID string, cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*client]struct{})
	}
	h.clients[userID][cl] = struct{}{}
}

func (h *Hub) removeClient(userID string, cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients := h.clients[userID]; clients != nil {
		delete(clients, cl)
		if len(clients) == 0 {
			delete(h.clients, userID)
		}
	}
	close(cl.send)
	_ = cl.conn.Close()
}

func (h *Hub) writeLoop(cl *client) {
	for event := range cl.send {
		if err := websocket.JSON.Send(cl.conn, event); err != nil {
			break
		}
	}
}

func (h *Hub) readLoop(cl *client) {
	defer cl.conn.Close()

	for {
		var payload interface{}
		if err := websocket.JSON.Receive(cl.conn, &payload); err != nil {
			break
		}
	}
}

// MarshalEvent converts an event payload into JSON bytes (utility for testing).
func MarshalEvent(event Event) ([]byte, error) {
	return json.Marshal(event)
}
