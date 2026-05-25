// Package transport builds the NetTransport a plugin uses to reach its target.
// Connectivity is orthogonal to protocol: "direct" dials out from the gateway,
// "agent" dials through a reverse tunnel registered by an in-target agent.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

// ErrAgentUnavailable is returned when an agent-transport connection has no live
// tunnel registered (the agent has not connected, or has gone offline).
var ErrAgentUnavailable = errors.New("transport: agent tunnel unavailable")

// DialFunc is a context-aware L4 dialer.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Direct dials the target straight from the gateway. It satisfies the L4 needs
// of socket/TCP protocols; an HTTP client is built by the plugin over
// DialContext, so HTTP() reports unavailable.
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

// HTTP reports ok=false: a direct-mode plugin builds its own HTTP client over
// DialContext. An L7 base URL + RoundTripper is only injected by an L7 agent.
func (d *Direct) HTTP() (string, http.RoundTripper, bool) {
	return "", nil, false
}

// TunnelRegistry resolves a live agent dialer for a connection.
type TunnelRegistry interface {
	Dialer(connectionID string) (DialFunc, bool)
}

// Registry is the in-memory tunnel registry. An agent that has dialed back
// registers its L4 dialer here under its connection id; Build resolves it for
// agent-mode connections. It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	dialers map[string]DialFunc
}

// NewRegistry returns an empty tunnel registry.
func NewRegistry() *Registry {
	return &Registry{dialers: make(map[string]DialFunc)}
}

// Register binds a connection's agent dialer; replacing any previous one.
func (r *Registry) Register(connectionID string, dial DialFunc) {
	r.mu.Lock()
	r.dialers[connectionID] = dial
	r.mu.Unlock()
}

// Remove drops a connection's tunnel (agent disconnected).
func (r *Registry) Remove(connectionID string) {
	r.mu.Lock()
	delete(r.dialers, connectionID)
	r.mu.Unlock()
}

// Dialer returns the registered dialer for a connection, if any.
func (r *Registry) Dialer(connectionID string) (DialFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dial, ok := r.dialers[connectionID]
	return dial, ok
}

// agentNet routes L4 through an agent tunnel dialer.
type agentNet struct {
	dial DialFunc
}

func (a *agentNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return a.dial(ctx, network, addr)
}

// HTTP reports ok=false: this transport provides L4 only. An L7 reverse-proxy
// agent supplies a base URL + RoundTripper for fat HTTP clients separately.
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
