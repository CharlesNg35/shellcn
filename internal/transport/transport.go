// Package transport builds the NetTransport a plugin uses to reach its target.
// Connectivity is orthogonal to protocol: "direct" dials out from the gateway,
// "agent" dials through a reverse tunnel (the tunnel itself lands in phase 4).
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

// ErrAgentUnavailable is returned when an agent-transport connection has no live
// tunnel yet (not enrolled, or the agent is offline).
var ErrAgentUnavailable = errors.New("transport: agent tunnel unavailable")

// DialFunc is a context-aware L4 dialer.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Direct dials the target straight from the gateway. It satisfies the L4 needs
// of socket/TCP protocols; L7 (HTTP) is the plugin's own concern in direct mode.
type Direct struct {
	dialer *net.Dialer
}

// NewDirect returns a direct transport with a sane dial timeout.
func NewDirect() *Direct {
	return &Direct{dialer: &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}}
}

// DialContext dials the requested address directly.
func (d *Direct) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, addr)
}

// HTTP reports ok=false: in direct mode a plugin builds its own HTTP client over
// DialContext. An L7 base URL + RoundTripper is only injected in agent L7 mode.
func (d *Direct) HTTP() (string, http.RoundTripper, bool) {
	return "", nil, false
}

// TunnelRegistry resolves a live agent dialer for a connection. The real
// implementation (reverse tunnels + enrollment) lands in phase 4; M1 ships an
// empty registry so agent-mode connections fail cleanly until then.
type TunnelRegistry interface {
	Dialer(connectionID string) (DialFunc, bool)
}

// EmptyTunnelRegistry has no tunnels — every agent lookup misses.
type EmptyTunnelRegistry struct{}

// Dialer always reports no tunnel in M1.
func (EmptyTunnelRegistry) Dialer(string) (DialFunc, bool) { return nil, false }

// agentNet routes L4 through an agent tunnel dialer.
type agentNet struct {
	dial DialFunc
}

func (a *agentNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return a.dial(ctx, network, addr)
}

// HTTP is wired when an L7 agent mode (e.g. k8s_reverse_proxy) lands (phase 6).
func (a *agentNet) HTTP() (string, http.RoundTripper, bool) {
	return "", nil, false
}

// Build returns the NetTransport for a connection based on its transport mode.
func Build(conn models.Connection, reg TunnelRegistry) (plugin.NetTransport, error) {
	switch conn.Transport {
	case "", string(plugin.TransportDirect):
		return NewDirect(), nil
	case string(plugin.TransportAgent):
		if reg == nil {
			return nil, ErrAgentUnavailable
		}
		dial, ok := reg.Dialer(conn.ID)
		if !ok {
			return nil, fmt.Errorf("%w: connection %q", ErrAgentUnavailable, conn.ID)
		}
		return &agentNet{dial: dial}, nil
	default:
		return nil, fmt.Errorf("transport: unknown mode %q", conn.Transport)
	}
}
