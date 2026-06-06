package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

// AgentHello is the first message an agent sends on the connect WebSocket,
// presenting its enrollment token (never in the URL or query). Forward advertises
// that the agent understands a per-stream target preamble (see WriteStreamTarget).
type AgentHello struct {
	Token   string `json:"token"`
	Forward bool   `json:"forward,omitempty"`
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
	// Forward: each L4 stream begins with a target preamble the agent dials
	// instead of Address. Set only when the plugin opted in and the agent supports it.
	Forward bool `json:"forward,omitempty"`
}

// WriteStreamTarget prefixes a forward-mode stream with the address the agent
// should dial: a 2-byte length then "network\x00addr".
func WriteStreamTarget(w io.Writer, network, addr string) error {
	payload := network + "\x00" + addr
	if len(payload) > 0xffff {
		return fmt.Errorf("stream target too long: %d bytes", len(payload))
	}
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := io.WriteString(w, payload)
	return err
}

// ReadStreamTarget reads a target preamble written by WriteStreamTarget.
func ReadStreamTarget(r io.Reader) (network, addr string, err error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", "", err
	}
	buf := make([]byte, binary.BigEndian.Uint16(hdr[:]))
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", "", err
	}
	netw, addr, ok := strings.Cut(string(buf), "\x00")
	if !ok {
		return "", "", fmt.Errorf("malformed stream target")
	}
	return netw, addr, nil
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
// It blocks until the tunnel is torn down. The bool return is true when this
// tunnel was still the active registration at close; false means it had already
// been replaced by a newer tunnel.
func ServeGatewayTunnel(ctx context.Context, c *websocket.Conn, connectionID string, reg *Registry, forward bool) (bool, error) {
	tunnelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		if err := KeepAliveWebSocket(tunnelCtx, c); err != nil {
			cancel()
			_ = c.CloseNow()
		}
	}()

	nc := websocket.NetConn(tunnelCtx, c, websocket.MessageBinary)

	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.LogOutput = io.Discard
	sess, err := yamux.Client(nc, cfg)
	if err != nil {
		return false, err
	}
	defer func() { _ = sess.Close() }()

	// In forward mode each opened stream names its dial target; otherwise the
	// agent proxies to its single declared Address.
	release := reg.Register(connectionID, func(_ context.Context, network, addr string) (net.Conn, error) {
		st, err := sess.Open()
		if err != nil {
			return nil, err
		}
		if forward {
			if err := WriteStreamTarget(st, network, addr); err != nil {
				_ = st.Close()
				return nil, err
			}
		}
		return st, nil
	})

	<-sess.CloseChan()
	return release(), nil
}
