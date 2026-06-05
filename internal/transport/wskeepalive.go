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
	ticker := time.NewTicker(websocketPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		pingCtx, cancel := context.WithTimeout(ctx, websocketPingTimeout)
		err := c.Ping(pingCtx)
		cancel()
		if err != nil {
			return err
		}
	}
}
