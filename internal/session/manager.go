// Package session is the in-memory session and channel registry.
package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/cluster"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

var (
	// ErrSessionClosed is returned when using a session that has been closed.
	ErrSessionClosed = errors.New("session: closed")
	// ErrSessionLimit is returned when a user is at their max session count.
	ErrSessionLimit = errors.New("session: per-user limit reached")
	// ErrChannelLimit is returned when a session is at its max channel count.
	ErrChannelLimit = errors.New("session: per-session channel limit reached")
)

// Key identifies one live session for an actor's scope on a connection.
type Key struct {
	ConnectionID string
	OwnerScope   string
}

// ConnectFunc lazily opens the upstream session on first use.
type ConnectFunc func(ctx context.Context) (plugin.Session, error)

// State is the lifecycle state of a registry entry.
type State string

const (
	StateConnecting State = "connecting"
	StateConnected  State = "connected"
	StateClosed     State = "closed"
	StateError      State = "error"
)

// Snapshot is a point-in-time view of one live registry entry.
type Snapshot struct {
	Key             Key
	UserID          string
	State           State
	Reason          string
	Channels        int
	Streams         int
	LastUsed        time.Time
	CreatedAt       time.Time
	LastHealthCheck time.Time
}

// Options bound the registry. Zero values fall back to sensible defaults.
type Options struct {
	IdleTimeout           time.Duration
	MaxSessionsPerUser    int
	MaxChannelsPerSession int
	HealthInterval        time.Duration
	FailureRetention      time.Duration
	OwnerRegistry         cluster.OwnerRegistry
	Instance              cluster.InstanceRef
	LeaseTTL              time.Duration
}

func (o Options) withDefaults() Options {
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = 15 * time.Minute
	}
	if o.MaxSessionsPerUser <= 0 {
		o.MaxSessionsPerUser = 50
	}
	if o.MaxChannelsPerSession <= 0 {
		o.MaxChannelsPerSession = 20
	}
	if o.HealthInterval <= 0 {
		o.HealthInterval = 30 * time.Second
	}
	if o.FailureRetention <= 0 {
		o.FailureRetention = 2 * time.Minute
	}
	if o.LeaseTTL <= 0 {
		o.LeaseTTL = 45 * time.Second
	}
	return o
}

type entry struct {
	mu              sync.Mutex
	key             Key
	userID          string
	sess            plugin.Session
	channels        int
	streams         int
	lastUsed        time.Time
	created         time.Time
	lastHealthCheck time.Time
	reason          string
	closed          bool
	lease           cluster.Lease
}

type failure struct {
	snapshot  Snapshot
	expiresAt time.Time
}

// Manager owns the session registry and lifecycle.
type Manager struct {
	mu       sync.Mutex
	sessions map[Key]*entry
	failures map[Key]failure
	opts     Options
	now      func() time.Time
	stop     chan struct{}
	wg       sync.WaitGroup
}

// New starts a manager and its background janitor.
func New(opts Options) *Manager {
	m := &Manager{
		sessions: make(map[Key]*entry),
		failures: make(map[Key]failure),
		opts:     opts.withDefaults(),
		now:      time.Now,
		stop:     make(chan struct{}),
	}
	m.wg.Add(1)
	go m.janitor()
	return m
}

// Acquire returns the live session for key, lazily connecting on first use.
func (m *Manager) Acquire(ctx context.Context, key Key, userID string, connect ConnectFunc) (*Handle, error) {
	m.mu.Lock()
	e, ok := m.sessions[key]
	if !ok {
		if n := m.countUser(userID); n >= m.opts.MaxSessionsPerUser {
			m.mu.Unlock()
			return nil, ErrSessionLimit
		}
		now := m.now()
		var lease cluster.Lease
		if m.opts.OwnerRegistry != nil {
			var err error
			lease, err = m.opts.OwnerRegistry.Claim(ctx, cluster.SessionOwnerKey(key.ConnectionID, key.OwnerScope), m.opts.Instance, cluster.ClaimOptions{
				Mode: cluster.ClaimExclusive,
				TTL:  m.opts.LeaseTTL,
			})
			if err != nil {
				m.mu.Unlock()
				return nil, err
			}
		}
		e = &entry{key: key, userID: userID, lastUsed: now, created: now, lease: lease}
		m.sessions[key] = e
		delete(m.failures, key)
	}
	m.mu.Unlock()

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, ErrSessionClosed
	}
	if e.sess == nil {
		sess, err := connect(ctx)
		now := m.now()
		if err != nil {
			e.lastUsed = now
			e.lastHealthCheck = now
			e.reason = err.Error()
			snap := e.snapshotLocked(StateError)
			e.closed = true
			lease := e.lease
			e.lease = nil
			e.mu.Unlock()
			m.removeAndRememberFailure(key, e, snap)
			if lease != nil {
				_ = lease.Release(context.Background())
			}
			return nil, err
		}
		if err := sess.HealthCheck(ctx); err != nil {
			_ = sess.Close()
			e.lastUsed = now
			e.lastHealthCheck = now
			e.reason = err.Error()
			snap := e.snapshotLocked(StateError)
			e.closed = true
			lease := e.lease
			e.lease = nil
			e.mu.Unlock()
			m.removeAndRememberFailure(key, e, snap)
			if lease != nil {
				_ = lease.Release(context.Background())
			}
			return nil, err
		}
		e.sess = sess
		e.lastHealthCheck = now
		e.reason = ""
	}
	e.lastUsed = m.now()
	e.mu.Unlock()
	return &Handle{m: m, e: e}, nil
}

// Status returns a snapshot for key without creating or connecting a session.
func (m *Manager) Status(key Key) (Snapshot, bool) {
	m.mu.Lock()
	e, ok := m.sessions[key]
	if !ok {
		if f, found := m.failures[key]; found {
			now := m.now()
			if now.Before(f.expiresAt) {
				m.mu.Unlock()
				return f.snapshot, true
			}
			delete(m.failures, key)
		}
	}
	m.mu.Unlock()
	if !ok {
		return Snapshot{}, false
	}
	return e.snapshot(), true
}

func (e *entry) snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked("")
}

func (e *entry) snapshotLocked(force State) Snapshot {
	state := StateConnected
	if force != "" {
		state = force
	} else if e.closed {
		state = StateClosed
	} else if e.sess == nil {
		state = StateConnecting
	}
	return Snapshot{
		Key: e.key, UserID: e.userID, State: state, Reason: e.reason,
		Channels: e.channels, Streams: e.streams,
		LastUsed: e.lastUsed, CreatedAt: e.created, LastHealthCheck: e.lastHealthCheck,
	}
}

// countUser counts a user's live sessions (caller holds m.mu).
func (m *Manager) countUser(userID string) int {
	n := 0
	for _, e := range m.sessions {
		if e.userID == userID {
			n++
		}
	}
	return n
}

func (m *Manager) removeAndRememberFailure(key Key, e *entry, snap Snapshot) {
	m.mu.Lock()
	if cur, ok := m.sessions[key]; ok && cur == e {
		delete(m.sessions, key)
	}
	m.failures[key] = failure{snapshot: snap, expiresAt: m.now().Add(m.opts.FailureRetention)}
	m.mu.Unlock()
}

// Close closes and removes the session for key, if present.
func (m *Manager) Close(key Key) {
	m.mu.Lock()
	e, ok := m.sessions[key]
	if ok {
		delete(m.sessions, key)
	}
	delete(m.failures, key)
	m.mu.Unlock()
	if ok {
		e.shutdown()
	}
}

// CloseConnection closes and removes every live session for a connection across
// all owner scopes. Callers use this after connection config changes so cached
// plugin options cannot outlive the saved connection state.
func (m *Manager) CloseConnection(connectionID string) {
	m.mu.Lock()
	entries := make([]*entry, 0)
	for key, e := range m.sessions {
		if key.ConnectionID == connectionID {
			entries = append(entries, e)
			delete(m.sessions, key)
		}
	}
	for key := range m.failures {
		if key.ConnectionID == connectionID {
			delete(m.failures, key)
		}
	}
	m.mu.Unlock()
	for _, e := range entries {
		e.shutdown()
	}
}

// Shutdown stops the janitor and closes every live session.
func (m *Manager) Shutdown() {
	close(m.stop)
	m.wg.Wait()
	m.mu.Lock()
	entries := make([]*entry, 0, len(m.sessions))
	for k, e := range m.sessions {
		entries = append(entries, e)
		delete(m.sessions, k)
	}
	m.failures = make(map[Key]failure)
	m.mu.Unlock()
	for _, e := range entries {
		e.shutdown()
	}
}

// Stats is a point-in-time snapshot for telemetry.
type Stats struct {
	Sessions int
	Channels int
}

// Stats returns the current session/channel counts.
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Stats{Sessions: len(m.sessions)}
	for _, e := range m.sessions {
		e.mu.Lock()
		s.Channels += e.channels
		e.mu.Unlock()
	}
	return s
}

// IdleTimeout returns the configured idle session timeout.
func (m *Manager) IdleTimeout() time.Duration {
	return m.opts.IdleTimeout
}

func (e *entry) shutdown() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	sess := e.sess
	lease := e.lease
	e.sess = nil
	e.lease = nil
	e.mu.Unlock()
	if sess != nil {
		_ = sess.Close()
	}
	if lease != nil {
		_ = lease.Release(context.Background())
	}
}

func (m *Manager) janitor() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.opts.HealthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.sweep()
		}
	}
}

// sweep reclaims idle sessions and drops upstreams that fail their health check.
func (m *Manager) sweep() {
	m.mu.Lock()
	m.pruneFailuresLocked(m.now())
	snapshot := make([]*entry, 0, len(m.sessions))
	for _, e := range m.sessions {
		snapshot = append(snapshot, e)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, e := range snapshot {
		e.mu.Lock()
		idle := e.channels == 0 && e.streams == 0 && m.now().Sub(e.lastUsed) > m.opts.IdleTimeout
		sess := e.sess
		closed := e.closed
		lease := e.lease
		e.mu.Unlock()

		if closed {
			continue
		}
		checkedAt := m.now()
		if lease != nil {
			if err := lease.Renew(ctx); err != nil {
				m.failEntry(e, err, checkedAt)
				continue
			}
		}
		if idle {
			m.Close(e.key)
			continue
		}
		if sess == nil {
			continue
		}
		if err := sess.HealthCheck(ctx); err != nil {
			m.failEntry(e, err, checkedAt)
			continue
		}
		e.mu.Lock()
		if !e.closed {
			e.lastHealthCheck = checkedAt
			e.reason = ""
		}
		e.mu.Unlock()
	}
}

func (m *Manager) pruneFailuresLocked(now time.Time) {
	for key, f := range m.failures {
		if !now.Before(f.expiresAt) {
			delete(m.failures, key)
		}
	}
}

func (m *Manager) failEntry(e *entry, err error, checkedAt time.Time) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.lastUsed = checkedAt
	e.lastHealthCheck = checkedAt
	e.reason = err.Error()
	snap := e.snapshotLocked(StateError)
	e.closed = true
	sess := e.sess
	lease := e.lease
	e.sess = nil
	e.lease = nil
	e.mu.Unlock()

	m.removeAndRememberFailure(e.key, e, snap)
	if sess != nil {
		_ = sess.Close()
	}
	if lease != nil {
		_ = lease.Release(context.Background())
	}
}
