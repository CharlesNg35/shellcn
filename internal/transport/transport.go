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
	"github.com/charlesng35/shellcn/internal/livelease"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ErrAgentUnavailable is returned when an agent-transport connection has no live
// tunnel registered (the agent has not connected, or has gone offline).
var ErrAgentUnavailable = errors.New("transport: agent tunnel unavailable")

// DialFunc is a context-aware L4 dialer.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Direct dials the target straight from the gateway.
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

// HTTP reports ok=false for direct transport.
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

// Registry is the concurrent in-memory tunnel registry.
type Registry struct {
	mu            sync.RWMutex
	seq           uint64
	dialers       map[string]registration
	leaseRegistry livelease.LeaseRegistry
	instance      livelease.InstanceRef
	leaseTTL      time.Duration
	renewInterval time.Duration
}

// registration tags a dialer with a unique id so a teardown only removes its
// own entry — never a later tunnel that replaced it.
type registration struct {
	id    uint64
	dial  DialFunc
	lease livelease.Lease
	stop  context.CancelFunc
}

type RegistryOption func(*Registry)

func WithLeaseRegistry(reg livelease.LeaseRegistry, instance livelease.InstanceRef) RegistryOption {
	return func(r *Registry) {
		r.leaseRegistry = reg
		r.instance = instance
	}
}

func WithLeaseTTL(ttl time.Duration) RegistryOption {
	return func(r *Registry) {
		r.leaseTTL = ttl
	}
}

func WithRenewInterval(interval time.Duration) RegistryOption {
	return func(r *Registry) {
		r.renewInterval = interval
	}
}

// NewRegistry returns an empty tunnel registry.
func NewRegistry(opts ...RegistryOption) *Registry {
	r := &Registry{dialers: make(map[string]registration), leaseTTL: 15 * time.Second, renewInterval: 5 * time.Second}
	for _, opt := range opts {
		opt(r)
	}
	if r.renewInterval <= 0 || r.renewInterval >= r.leaseTTL {
		r.renewInterval = r.leaseTTL / 3
	}
	return r
}

// TunnelRegistration is the lifecycle handle for one registered agent tunnel.
type TunnelRegistration struct {
	registry     *Registry
	connectionID string
	id           uint64
	lease        livelease.Lease
	stopRenewal  context.CancelFunc
}

// TunnelRelease reports what happened when a tunnel registration ended.
type TunnelRelease struct {
	// WasActive is true when this registration was still the active live-state lease.
	// False means a newer tunnel had already replaced it.
	WasActive bool
}

// Register binds an agent dialer for the lifetime of one tunnel.
func (r *Registry) Register(ctx context.Context, connectionID string, dial DialFunc) (*TunnelRegistration, error) {
	var lease livelease.Lease
	var stop context.CancelFunc
	if r.leaseRegistry != nil {
		var err error
		lease, err = r.leaseRegistry.Claim(ctx, livelease.AgentLeaseKey(connectionID), r.instance, livelease.ClaimOptions{
			Mode: livelease.ClaimReplace,
			TTL:  r.leaseTTL,
		})
		if err != nil {
			return nil, err
		}
		renewCtx, cancel := context.WithCancel(ctx)
		stop = cancel
		go renewLease(renewCtx, r.renewInterval, lease)
	}
	r.mu.Lock()
	r.seq++
	id := r.seq
	r.dialers[connectionID] = registration{id: id, dial: dial, lease: lease, stop: stop}
	r.mu.Unlock()
	return &TunnelRegistration{registry: r, connectionID: connectionID, id: id, lease: lease, stopRenewal: stop}, nil
}

// Release removes this tunnel registration and releases its live-state lease.
func (r *TunnelRegistration) Release() TunnelRelease {
	reg := r.registry
	lease := r.lease
	stop := r.stopRenewal

	reg.mu.Lock()
	removedLocal := false
	if cur, ok := reg.dialers[r.connectionID]; ok && cur.id == r.id {
		delete(reg.dialers, r.connectionID)
		removedLocal = true
		stop = cur.stop
		lease = cur.lease
	}
	reg.mu.Unlock()

	if stop != nil {
		stop()
	}
	wasActive := removedLocal
	if lease != nil && reg.leaseRegistry != nil {
		wasActive = false
		key := livelease.AgentLeaseKey(r.connectionID)
		if ref, ok, err := reg.leaseRegistry.Get(context.Background(), key); err == nil && ok {
			wasActive = ref.LeaseID == lease.Ref().LeaseID
		}
	}
	if lease != nil {
		_ = lease.Release(context.Background())
	}
	return TunnelRelease{WasActive: wasActive}
}

func renewLease(ctx context.Context, interval time.Duration, lease livelease.Lease) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := lease.Renew(ctx); err != nil {
				return
			}
		}
	}
}

// Remove drops a connection's tunnel unconditionally (e.g. on revocation).
func (r *Registry) Remove(connectionID string) {
	r.mu.Lock()
	reg, ok := r.dialers[connectionID]
	if ok {
		delete(r.dialers, connectionID)
	}
	r.mu.Unlock()
	if ok && reg.stop != nil {
		reg.stop()
	}
	if ok && reg.lease != nil {
		_ = reg.lease.Release(context.Background())
	}
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

// agentNet routes traffic through an agent tunnel dialer.
type agentNet struct {
	dial DialFunc
	mode plugin.AgentMode
}

func (a *agentNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := a.dial(ctx, network, addr)
	if err != nil {
		// A tunnel dial failure means the agent path is gone.
		return nil, fmt.Errorf("%w: %w", ErrAgentUnavailable, err)
	}
	return conn, nil
}

// HTTP returns an L7 base URL and RoundTripper for http_proxy-style modes.
// The "http" scheme is logical; the underlying path is the encrypted agent tunnel.
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

// Build returns the NetTransport for a connection.
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
