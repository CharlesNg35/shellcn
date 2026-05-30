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
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
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
	target targetAllowlist
}

// NewDirect returns a direct transport with a sane dial timeout.
func NewDirect() *Direct {
	return &Direct{dialer: &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}}
}

func NewDirectForConnection(conn models.Connection) *Direct {
	d := NewDirect()
	d.target = newTargetAllowlist(conn.Config)
	return d
}

// DialContext dials the requested address directly.
func (d *Direct) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := d.target.allow(network, addr); err != nil {
		return nil, err
	}
	return d.dialer.DialContext(ctx, network, addr)
}

// HTTP reports ok=false: a direct-mode plugin builds its own HTTP client over
// DialContext. An L7 base URL + RoundTripper is only injected by an L7 agent.
func (d *Direct) HTTP() (string, http.RoundTripper, bool) {
	return "", nil, false
}

type targetAllowlist struct {
	enabled    bool
	hosts      map[string]bool
	addrs      map[string]bool
	unix       map[string]bool
	ports      map[string]bool
	portRanges []portRange
}

type portRange struct {
	start int
	end   int
}

func newTargetAllowlist(config map[string]any) targetAllowlist {
	t := targetAllowlist{
		enabled: true,
		hosts:   map[string]bool{},
		addrs:   map[string]bool{},
		unix:    map[string]bool{},
		ports:   map[string]bool{},
	}
	rangeStarts := map[string]int{}
	rangeEnds := map[string]int{}
	for key, value := range config {
		key = strings.ToLower(key)
		if port, ok := portValue(value); ok && strings.Contains(key, "port") {
			t.ports[strconv.Itoa(port)] = true
			if base, ok := rangeStartKey(key); ok {
				rangeStarts[base] = port
			}
			if base, ok := rangeEndKey(key); ok {
				rangeEnds[base] = port
			}
			continue
		}
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		t.addString(s)
	}
	for base, start := range rangeStarts {
		if end, ok := rangeEnds[base]; ok {
			t.addPortRange(start, end)
		}
	}
	return t
}

func (t targetAllowlist) addString(raw string) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "/") {
		t.unix[s] = true
	}
	if u, err := url.Parse(s); err == nil && u.Hostname() != "" {
		t.addHost(u.Hostname())
		if u.Port() != "" {
			t.addAddr(u.Hostname(), u.Port())
			t.ports[u.Port()] = true
		}
		return
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		t.addHost(host)
		t.addAddr(host, port)
		t.ports[port] = true
		return
	}
	if !strings.ContainsAny(s, "/\\") {
		t.addHost(s)
	}
}

func (t targetAllowlist) addHost(host string) {
	if host == "" {
		return
	}
	t.hosts[host] = true
	for _, alias := range loopbackAliases(host) {
		t.hosts[alias] = true
	}
}

func (t targetAllowlist) addAddr(host, port string) {
	t.addrs[net.JoinHostPort(host, port)] = true
	for _, alias := range loopbackAliases(host) {
		t.addrs[net.JoinHostPort(alias, port)] = true
	}
}

func (t *targetAllowlist) addPortRange(start, end int) {
	if start < 1 || end < 1 || start > 65535 || end > 65535 {
		return
	}
	if end < start {
		start, end = end, start
	}
	t.portRanges = append(t.portRanges, portRange{start: start, end: end})
}

func loopbackAliases(host string) []string {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "localhost", "127.0.0.1", "::1":
		return []string{"localhost", "127.0.0.1", "::1"}
	default:
		return nil
	}
}

func (t targetAllowlist) allow(network, addr string) error {
	if !t.enabled {
		return nil
	}
	if len(t.hosts) == 0 && len(t.addrs) == 0 && len(t.unix) == 0 {
		return fmt.Errorf("transport: direct target is not declared in connection config")
	}
	switch network {
	case "unix", "unixpacket":
		if t.unix[addr] || t.addrs[addr] {
			return nil
		}
		return fmt.Errorf("transport: direct dial to %q is outside connection target", addr)
	default:
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("transport: direct dial address %q must include host and port", addr)
		}
		if t.addrs[net.JoinHostPort(host, port)] {
			return nil
		}
		if t.hosts[host] && (len(t.ports) == 0 || t.portAllowed(port)) {
			return nil
		}
		return fmt.Errorf("transport: direct dial to %q is outside connection target", addr)
	}
}

func (t targetAllowlist) portAllowed(port string) bool {
	if t.ports[port] {
		return true
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	for _, r := range t.portRanges {
		if n >= r.start && n <= r.end {
			return true
		}
	}
	return false
}

func portValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		if v >= 1 && v <= 65535 {
			return int(v), true
		}
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && n >= 1 && n <= 65535 {
			return n, true
		}
	}
	return 0, false
}

func rangeStartKey(key string) (string, bool) {
	for _, suffix := range []string{"_start", "_min"} {
		if strings.HasSuffix(key, suffix) {
			return strings.TrimSuffix(key, suffix), true
		}
	}
	return "", false
}

func rangeEndKey(key string) (string, bool) {
	for _, suffix := range []string{"_end", "_max"} {
		if strings.HasSuffix(key, suffix) {
			return strings.TrimSuffix(key, suffix), true
		}
	}
	return "", false
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
	seq     uint64
	dialers map[string]registration
}

// registration tags a dialer with a unique id so a teardown only removes its
// own entry — never a later tunnel that replaced it.
type registration struct {
	id   uint64
	dial DialFunc
}

// NewRegistry returns an empty tunnel registry.
func NewRegistry() *Registry {
	return &Registry{dialers: make(map[string]registration)}
}

// Register binds a connection's agent dialer, replacing any previous one, and
// returns a release func that removes only this registration. A teardown that
// fires after another tunnel has replaced this one is a no-op, so it cannot
// drop the live tunnel.
func (r *Registry) Register(connectionID string, dial DialFunc) (release func()) {
	r.mu.Lock()
	r.seq++
	id := r.seq
	r.dialers[connectionID] = registration{id: id, dial: dial}
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		if cur, ok := r.dialers[connectionID]; ok && cur.id == id {
			delete(r.dialers, connectionID)
		}
		r.mu.Unlock()
	}
}

// Remove drops a connection's tunnel unconditionally (e.g. on revocation).
func (r *Registry) Remove(connectionID string) {
	r.mu.Lock()
	delete(r.dialers, connectionID)
	r.mu.Unlock()
}

// Dialer returns the registered dialer for a connection, if any.
func (r *Registry) Dialer(connectionID string) (DialFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.dialers[connectionID]
	return reg.dial, ok
}

// agentL7Host is the sentinel authority an L7 (http_proxy) agent client targets.
// The agent ignores it and proxies to its declared upstream; the gateway-side
// transport dials the tunnel regardless of this value.
const agentL7Host = app.AgentInternalHost

// agentNet routes traffic through an agent tunnel dialer. For L4 modes
// (tcp/unix) it exposes DialContext; for the L7 http_proxy mode it additionally
// exposes a base URL + RoundTripper so fat HTTP clients (client-go) can reach an
// upstream the gateway cannot dial, with credential injection done agent-side.
type agentNet struct {
	dial DialFunc
	mode plugin.AgentMode
}

func (a *agentNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := a.dial(ctx, network, addr)
	if err != nil {
		// A dial failure through the tunnel means the agent path is gone (e.g.
		// yamux "session shutdown" after the agent disconnected).
		return nil, fmt.Errorf("%w: %w", ErrAgentUnavailable, err)
	}
	return conn, nil
}

// HTTP returns an L7 base URL + RoundTripper for http_proxy-style modes (else ok=false).
// The "http" scheme is logical: DialContext opens a yamux stream over the agent's
// already-encrypted wss tunnel, which re-originates to the upstream over https —
// so an inner TLS layer would only be TLS-in-TLS with no gain.
func (a *agentNet) HTTP() (string, http.RoundTripper, bool) {
	if a.mode != plugin.AgentHTTP && a.mode != plugin.AgentHostMonitor {
		return "", nil, false
	}
	rt := &http.Transport{
		DialContext:           a.DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 0,
	}
	return "http://" + agentL7Host, rt, true
}

// Build returns the NetTransport for a connection based on its transport mode.
// agentMode is the plugin's declared agent proxy mode; it selects whether an
// agent-transport connection exposes an L7 HTTP() endpoint (http_proxy) or only
// L4 DialContext (tcp/unix). It is ignored for direct transport.
func Build(conn models.Connection, reg TunnelRegistry, agentMode plugin.AgentMode) (plugin.NetTransport, error) {
	switch conn.Transport {
	case "", string(plugin.TransportDirect):
		return NewDirectForConnection(conn), nil
	case string(plugin.TransportAgent):
		if reg == nil {
			return nil, ErrAgentUnavailable
		}
		dial, ok := reg.Dialer(conn.ID)
		if !ok {
			return nil, fmt.Errorf("%w: connection %q", ErrAgentUnavailable, conn.ID)
		}
		return &agentNet{dial: dial, mode: agentMode}, nil
	default:
		return nil, fmt.Errorf("transport: unknown mode %q", conn.Transport)
	}
}
