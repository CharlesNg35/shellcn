package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

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
	AgentModeUDP         = "udp"
	AgentModeHTTP        = "http_proxy"
	AgentModeHostMonitor = "host_monitor"
)

// IsUDPNetwork reports whether a dial network is UDP; such streams are
// datagram-framed over the tunnel, not copied as a byte stream.
func IsUDPNetwork(network string) bool {
	return network == "udp" || network == "udp4" || network == "udp6"
}

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

// maxDatagram is the largest framed payload the 2-byte length prefix allows.
const maxDatagram = 0xffff

// WriteDatagram frames one datagram (2-byte length + payload) in a single Write
// so concurrent senders can't interleave header and body.
func WriteDatagram(w io.Writer, p []byte) error {
	if len(p) > maxDatagram {
		return fmt.Errorf("datagram too large: %d bytes", len(p))
	}
	frame := make([]byte, 2+len(p))
	binary.BigEndian.PutUint16(frame, uint16(len(p)))
	copy(frame[2:], p)
	n, err := w.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// ReadDatagram reads one frame written by WriteDatagram.
func ReadDatagram(r io.Reader) ([]byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	buf := make([]byte, binary.BigEndian.Uint16(hdr[:]))
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// datagramConn gives a byte-stream tunnel stream connected-UDP semantics: one
// framed datagram per Read/Write. Deadlines/Close/addresses delegate to the stream.
type datagramConn struct {
	net.Conn
	writeMu sync.Mutex
}

// NewDatagramConn wraps a tunnel stream with datagram framing.
func NewDatagramConn(c net.Conn) net.Conn { return &datagramConn{Conn: c} }

func (d *datagramConn) Write(p []byte) (int, error) {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if err := WriteDatagram(d.Conn, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (d *datagramConn) Read(p []byte) (int, error) {
	dg, err := ReadDatagram(d.Conn)
	if err != nil {
		return 0, err
	}
	return copy(p, dg), nil // short buffer truncates, like *net.UDPConn.Read
}

// AgentConnectResponse is the gateway's reply to AgentHello. On OK the tunnel
// switches to multiplexed streaming; otherwise the agent disconnects.
type AgentConnectResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// Retryable marks an OK:false failure as transient (gateway-side): the agent
	// reconnects instead of treating its token as permanently rejected.
	Retryable bool             `json:"retryable,omitempty"`
	Proxy     AgentProxyTarget `json:"proxy,omitzero"`
}

// ServeGatewayTunnel runs the gateway side of an agent tunnel over an already
// authenticated WebSocket. The gateway is the yamux *client* (it opens a stream
// per upstream dial); the in-target agent is the yamux *server* (it accepts each
// stream and proxies it to the declared target). The connection's dialer is
// registered for the lifetime of the tunnel and removed when it closes.
//
// It blocks until the tunnel is torn down. onRegistered runs after the dialer is
// registered and before the function starts waiting for tunnel teardown. The
// bool return is true when this tunnel was still the active registration at
// close; false means it had already been replaced by a newer tunnel.
func ServeGatewayTunnel(ctx context.Context, c *websocket.Conn, connectionID string, reg *Registry, forward bool, onRegistered func() error) (bool, error) {
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
	registration, err := reg.Register(ctx, connectionID, func(_ context.Context, network, addr string) (net.Conn, error) {
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
		if IsUDPNetwork(network) {
			return NewDatagramConn(st), nil
		}
		return st, nil
	})
	if err != nil {
		_ = sess.Close()
		return false, err
	}
	if onRegistered != nil {
		if err := onRegistered(); err != nil {
			released := registration.Release()
			_ = sess.Close()
			return released.WasActive, err
		}
	}

	<-sess.CloseChan()
	return registration.Release().WasActive, nil
}
