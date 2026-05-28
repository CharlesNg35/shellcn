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

// Agent proxy mode wire values, mirroring plugin.AgentMode. They live here so
// the standalone agent binary depends only on this package, not internal/plugin.
const (
	AgentModeTCP         = "tcp"
	AgentModeUnix        = "unix"
	AgentModeHTTP        = "http_proxy"
	AgentModeHostMonitor = "host_monitor"
)

// AgentProxyTarget tells the agent what local endpoint to expose back. The L7
// (http_proxy) fields are generic credential-injection knobs: a token file to
// turn into a bearer header and a CA file to verify the upstream — no protocol
// vocabulary, so the agent stays plugin-agnostic.
type AgentProxyTarget struct {
	Mode      string `json:"mode"`
	Address   string `json:"address"`
	TokenFile string `json:"tokenFile,omitempty"`
	CAFile    string `json:"caFile,omitempty"`
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

	release := reg.Register(connectionID, func(_ context.Context, _, _ string) (net.Conn, error) {
		return sess.Open()
	})
	defer release()

	<-sess.CloseChan()
	return nil
}
