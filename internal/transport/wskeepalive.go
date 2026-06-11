package transport

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

const (
	websocketPingInterval = 25 * time.Second
	websocketPingTimeout  = 10 * time.Second
)

// KeepAliveWebSocket sends WebSocket control pings until ctx is cancelled.
// Callers must keep reading from c concurrently so pong frames are processed.
func KeepAliveWebSocket(ctx context.Context, c *websocket.Conn) error {
	return KeepAliveWebSocketWhenIdle(ctx, c, nil)
}

// KeepAliveWebSocketWhenIdle sends WebSocket control pings only after the stream
// has been idle for at least one ping interval. Active terminal/desktop streams
// can produce enough traffic that an extra control ping competes with data writes
// and may time out under backpressure.
func KeepAliveWebSocketWhenIdle(ctx context.Context, c *websocket.Conn, lastActive func() time.Time) error {
	ticker := time.NewTicker(websocketPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if lastActive != nil && !shouldPing(lastActive(), time.Now(), websocketPingInterval) {
			continue
		}
		pingCtx, cancel := context.WithTimeout(ctx, websocketPingTimeout)
		err := c.Ping(pingCtx)
		cancel()
		if err != nil {
			return err
		}
	}
}

func shouldPing(lastActive, now time.Time, idleFor time.Duration) bool {
	return lastActive.IsZero() || !now.Before(lastActive.Add(idleFor))
}
