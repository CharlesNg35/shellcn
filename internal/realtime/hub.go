package realtime

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1 << 20 // 1 MiB

	defaultBufferSize = 64
)

// Message represents a JSON payload delivered to realtime subscribers.
type Message struct {
	Stream string         `json:"stream"`
	Event  string         `json:"event"`
	Data   any            `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type controlMessage struct {
	Action  string   `json:"action"`
	Streams []string `json:"streams"`
}

// Hub coordinates multiplexed realtime streams for connected clients.
type Hub struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]map[*connection]struct{}
	upgrader      websocket.Upgrader
}

// NewHub constructs a realtime hub.
func NewHub() *Hub {
	return &Hub{
		subscriptions: make(map[string]map[string]map[*connection]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				// Allow same-origin requests and explicit localhost development.
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				originHost := hostWithoutPort(origin)
				requestHost := hostWithoutPort(r.Host)
				return originHost == requestHost || isLoopback(originHost)
			},
		},
	}
}

// Serve upgrades the HTTP connection to a WebSocket and registers the client with the provided streams.
// The allowed set can be nil to indicate all streams are permitted.
func (h *Hub) Serve(userID string, streams []string, allowed map[string]struct{}, w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("realtime: upgrade failed: %v", err)
		return
	}

	client := newConnection(h, conn, userID, allowed)
	h.subscribe(client, streams)

	go client.writeLoop()
	client.readLoop()
}

// BroadcastToUser delivers a message to all connections for the supplied user on a stream.
func (h *Hub) BroadcastToUser(stream, userID string, message Message) {
	stream = normalizeStream(stream)
	if stream == "" || userID == "" {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clientsByUser, ok := h.subscriptions[stream]
	if !ok {
		return
	}

	targets := clientsByUser[userID]
	if len(targets) == 0 {
		return
	}

	message.Stream = stream
	for client := range targets {
		h.enqueue(client, message)
	}
}

// BroadcastToUsers delivers a message to each of the supplied user IDs on the provided stream.
func (h *Hub) BroadcastToUsers(stream string, userIDs []string, message Message) {
	for _, userID := range userIDs {
		h.BroadcastToUser(stream, userID, message)
	}
}

// BroadcastStream delivers a message to every subscriber listening on the provided stream.
func (h *Hub) BroadcastStream(stream string, message Message) {
	stream = normalizeStream(stream)
	if stream == "" {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clientsByUser, ok := h.subscriptions[stream]
	if !ok {
		return
	}

	message.Stream = stream
	for _, clients := range clientsByUser {
		for client := range clients {
			h.enqueue(client, message)
		}
	}
}

func (h *Hub) subscribe(client *connection, streams []string) {
	if len(streams) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, stream := range uniqueStreams(streams) {
		if stream == "" {
			continue
		}
		if !client.isAllowed(stream) {
			log.Printf("realtime: ignoring unauthorized stream '%s' for user=%s", stream, client.userID)
			continue
		}
		if client.streams == nil {
			client.streams = make(map[string]struct{})
		}
		if _, exists := client.streams[stream]; exists {
			continue
		}

		if h.subscriptions[stream] == nil {
			h.subscriptions[stream] = make(map[string]map[*connection]struct{})
		}
		if h.subscriptions[stream][client.userID] == nil {
			h.subscriptions[stream][client.userID] = make(map[*connection]struct{})
		}

		client.streams[stream] = struct{}{}
		h.subscriptions[stream][client.userID][client] = struct{}{}
	}
}

func (h *Hub) unsubscribe(client *connection, streams []string) {
	if len(streams) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, stream := range uniqueStreams(streams) {
		if stream == "" {
			continue
		}
		h.removeSubscriptionLocked(client, stream, false)
	}
}

func (h *Hub) unregister(client *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for stream := range client.streams {
		h.removeSubscriptionLocked(client, stream, true)
	}
}

func (h *Hub) removeSubscriptionLocked(client *connection, stream string, removeAll bool) {
	stream = normalizeStream(stream)
	if stream == "" {
		return
	}

	clientsByUser, ok := h.subscriptions[stream]
	if !ok {
		return
	}

	userClients := clientsByUser[client.userID]
	if len(userClients) == 0 {
		return
	}

	delete(userClients, client)
	if len(userClients) == 0 {
		delete(clientsByUser, client.userID)
	}
	if len(clientsByUser) == 0 {
		delete(h.subscriptions, stream)
	}

	if removeAll {
		delete(client.streams, stream)
	}
}

func (h *Hub) enqueue(client *connection, message Message) {
	select {
	case client.send <- message:
	default:
		log.Printf("realtime: dropping backpressure client (user=%s)", client.userID)
		client.close()
	}
}

type connection struct {
	hub     *Hub
	socket  *websocket.Conn
	userID  string
	streams map[string]struct{}
	send    chan Message
	once    sync.Once
	allowed map[string]struct{}
}

func newConnection(hub *Hub, conn *websocket.Conn, userID string, allowed map[string]struct{}) *connection {
	return &connection{
		hub:     hub,
		socket:  conn,
		userID:  userID,
		send:    make(chan Message, defaultBufferSize),
		allowed: allowed,
	}
}

func (c *connection) readLoop() {
	defer c.close()

	c.socket.SetReadLimit(maxMessageSize)
	_ = c.socket.SetReadDeadline(time.Now().Add(pongWait))
	c.socket.SetPongHandler(func(string) error {
		_ = c.socket.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, payload, err := c.socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("realtime: unexpected close for user=%s: %v", c.userID, err)
			}
			break
		}

		if len(payload) == 0 {
			continue
		}

		var ctrl controlMessage
		if err := json.Unmarshal(payload, &ctrl); err != nil {
			log.Printf("realtime: invalid control payload for user=%s: %v", c.userID, err)
			continue
		}

		switch strings.ToLower(strings.TrimSpace(ctrl.Action)) {
		case "subscribe":
			c.hub.subscribe(c, ctrl.Streams)
		case "unsubscribe":
			c.hub.unsubscribe(c, ctrl.Streams)
		case "ping":
			// Clients can send ping control messages; reply with pong.
			c.send <- Message{Stream: "", Event: "pong"}
		default:
			log.Printf("realtime: unsupported control action '%s' for user=%s", ctrl.Action, c.userID)
		}
	}
}

func (c *connection) writeLoop() {
	defer c.close()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.socket.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *connection) close() {
	c.once.Do(func() {
		c.hub.unregister(c)
		close(c.send)
		_ = c.socket.Close()
	})
}

func (c *connection) isAllowed(stream string) bool {
	if len(c.allowed) == 0 {
		return true
	}
	_, ok := c.allowed[stream]
	return ok
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		parsed, err := http.NewRequest(http.MethodGet, host, nil)
		if err == nil {
			return hostWithoutPort(parsed.URL.Host)
		}
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}

func normalizeStream(stream string) string {
	return strings.ToLower(strings.TrimSpace(stream))
}

func uniqueStreams(streams []string) []string {
	unique := make(map[string]struct{}, len(streams))
	var result []string
	for _, stream := range streams {
		if stream = normalizeStream(stream); stream != "" {
			if _, exists := unique[stream]; !exists {
				unique[stream] = struct{}{}
				result = append(result, stream)
			}
		}
	}
	return result
}
