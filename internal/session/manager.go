// Package session is the in-memory session + channel registry. A session is a
// live, authenticated runtime for one connection; channels are the tracked
// streams inside it. State lives in memory and is not shared across instances.
package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
)

var (
	// ErrSessionClosed is returned when using a session that has been closed.
	ErrSessionClosed = errors.New("session: closed")
	// ErrSessionLimit is returned when a user is at their max session count.
	ErrSessionLimit = errors.New("session: per-user limit reached")
	// ErrChannelLimit is returned when a session is at its max channel count.
	ErrChannelLimit = errors.New("session: per-session channel limit reached")
)

// Key identifies a session: one connection within an owner scope (per-user, or
// "shared" for a shared connection that reuses one upstream).
type Key struct {
	ConnectionID string
	OwnerScope   string
}

// ConnectFunc lazily opens the upstream session on first use. The caller builds
// the ConnectConfig (decrypt secrets, resolve credential, wire transport).
type ConnectFunc func(ctx context.Context) (plugin.Session, error)

// Options bound the registry. Zero values fall back to sensible defaults.
type Options struct {
	IdleTimeout           time.Duration
	MaxSessionsPerUser    int
	MaxChannelsPerSession int
	HealthInterval        time.Duration
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
	return o
}

type entry struct {
	mu       sync.Mutex
	key      Key
	userID   string
	sess     plugin.Session
	channels int
	lastUsed time.Time
	closed   bool
}

// Manager owns the session registry and its lifecycle (lazy connect, idle
// reclaim, periodic health check, graceful shutdown).
type Manager struct {
	mu       sync.Mutex
	sessions map[Key]*entry
	opts     Options
	now      func() time.Time
	stop     chan struct{}
	wg       sync.WaitGroup
}

// New starts a manager and its background janitor.
func New(opts Options) *Manager {
	m := &Manager{
		sessions: make(map[Key]*entry),
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
		e = &entry{key: key, userID: userID, lastUsed: m.now()}
		m.sessions[key] = e
	}
	m.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrSessionClosed
	}
	if e.sess == nil {
		sess, err := connect(ctx)
		if err != nil {
			m.remove(key, e)
			return nil, err
		}
		e.sess = sess
	}
	e.lastUsed = m.now()
	return &Handle{m: m, e: e}, nil
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

func (m *Manager) remove(key Key, e *entry) {
	m.mu.Lock()
	if cur, ok := m.sessions[key]; ok && cur == e {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
}

// Close closes and removes the session for key, if present.
func (m *Manager) Close(key Key) {
	m.mu.Lock()
	e, ok := m.sessions[key]
	if ok {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
	if ok {
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

func (e *entry) shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	if e.sess != nil {
		_ = e.sess.Close()
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
	snapshot := make([]*entry, 0, len(m.sessions))
	for _, e := range m.sessions {
		snapshot = append(snapshot, e)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, e := range snapshot {
		e.mu.Lock()
		idle := e.channels == 0 && m.now().Sub(e.lastUsed) > m.opts.IdleTimeout
		sess := e.sess
		closed := e.closed
		e.mu.Unlock()

		if closed {
			continue
		}
		if idle {
			m.Close(e.key)
			continue
		}
		if sess != nil && sess.HealthCheck(ctx) != nil {
			// Dead upstream: closing the session closes its channels, which ends
			// any WS stream and prompts the UI to offer reconnect.
			m.Close(e.key)
		}
	}
}
