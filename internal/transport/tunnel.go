package transport

import (
	"context"
	"io"
	"net"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

// AgentHello is the first message an agent sends on the connect WebSocket,
// presenting its enrollment token (never in the URL or query).
type AgentHello struct {
	Token string `json:"token"`
}

// AgentProxyTarget tells the agent what local endpoint to expose back.
type AgentProxyTarget struct {
	Mode    string `json:"mode"`
	Address string `json:"address"`
}

// AgentConnectResponse is the gateway's reply to AgentHello. On OK the tunnel
// switches to multiplexed streaming; otherwise the agent disconnects.
type AgentConnectResponse struct {
	OK    bool             `json:"ok"`
	Error string           `json:"error,omitempty"`
	Proxy AgentProxyTarget `json:"proxy,omitzero"`
}

// ServeGatewayTunnel runs the gateway side of an agent tunnel over an already
// authenticated WebSocket. The gateway is the yamux *client* (it opens a stream
// per upstream dial); the in-target agent is the yamux *server* (it accepts each
// stream and proxies it to the declared target). The connection's dialer is
// registered for the lifetime of the tunnel and removed when it closes.
//
// It blocks until the tunnel is torn down.
func ServeGatewayTunnel(c *websocket.Conn, connectionID string, reg *Registry) error {
	nc := websocket.NetConn(context.Background(), c, websocket.MessageBinary)

	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.LogOutput = io.Discard
	sess, err := yamux.Client(nc, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = sess.Close() }()

	reg.Register(connectionID, func(_ context.Context, _, _ string) (net.Conn, error) {
		return sess.Open()
	})
	defer reg.Remove(connectionID)

	<-sess.CloseChan()
	return nil
}
