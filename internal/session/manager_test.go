package session_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/session"
)

// fakeSession is a controllable plugin.Session for tests.
type fakeSession struct {
	mu        sync.Mutex
	closed    bool
	healthErr error
}

func (f *fakeSession) HealthCheck(context.Context) error { return f.healthErr }

func (f *fakeSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return &fakeChannel{}, nil
}

func (f *fakeSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeSession) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

type fakeChannel struct{ closed atomic.Bool }

func (c *fakeChannel) Read([]byte) (int, error)    { return 0, io.EOF }
func (c *fakeChannel) Write(p []byte) (int, error) { return len(p), nil }
func (c *fakeChannel) Close() error                { c.closed.Store(true); return nil }
func (c *fakeChannel) Kind() plugin.StreamKind     { return plugin.StreamTerminal }

func connector(s plugin.Session, hits *int32) session.ConnectFunc {
	return func(context.Context) (plugin.Session, error) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		return s, nil
	}
}

func TestAcquireLazyConnectAndReuse(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()

	var hits int32
	fs := &fakeSession{}
	key := session.Key{ConnectionID: "c1", OwnerScope: "u1"}

	h1, err := m.Acquire(context.Background(), key, "u1", connector(fs, &hits))
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	// Second acquire reuses the same upstream — connect runs only once.
	h2, err := m.Acquire(context.Background(), key, "u1", connector(fs, &hits))
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	if h1.Session() != h2.Session() {
		t.Error("expected the same session reused")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("connect should run once (lazy + cached), ran %d times", hits)
	}
	if s := m.Stats(); s.Sessions != 1 {
		t.Errorf("stats sessions: want 1, got %d", s.Sessions)
	}
}

func TestConnectErrorNotCached(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", OwnerScope: "u1"}

	boom := errors.New("dial failed")
	_, err := m.Acquire(context.Background(), key, "u1", func(context.Context) (plugin.Session, error) {
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want dial error, got %v", err)
	}
	if s := m.Stats(); s.Sessions != 0 {
		t.Errorf("failed connect must not leave an entry: %d sessions", s.Sessions)
	}
	// A subsequent successful acquire works (entry was cleaned up).
	if _, err := m.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil)); err != nil {
		t.Errorf("retry after failure: %v", err)
	}
}

func TestChannelTrackingAndLimit(t *testing.T) {
	m := session.New(session.Options{MaxChannelsPerSession: 2})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", OwnerScope: "u1"}
	h, _ := m.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil))

	c1, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamTerminal})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	if _, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{}); err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if s := m.Stats(); s.Channels != 2 {
		t.Errorf("channels open: want 2, got %d", s.Channels)
	}
	// Third exceeds the cap.
	if _, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{}); !errors.Is(err, session.ErrChannelLimit) {
		t.Errorf("want ErrChannelLimit, got %v", err)
	}
	// Closing a channel frees a slot.
	_ = c1.Close()
	if s := m.Stats(); s.Channels != 1 {
		t.Errorf("after close: want 1 channel, got %d", s.Channels)
	}
	if _, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{}); err != nil {
		t.Errorf("open after freeing a slot: %v", err)
	}
}

func TestPerUserSessionLimit(t *testing.T) {
	m := session.New(session.Options{MaxSessionsPerUser: 1})
	defer m.Shutdown()
	_, err := m.Acquire(context.Background(), session.Key{ConnectionID: "a", OwnerScope: "u1"}, "u1", connector(&fakeSession{}, nil))
	if err != nil {
		t.Fatalf("first session: %v", err)
	}
	_, err = m.Acquire(context.Background(), session.Key{ConnectionID: "b", OwnerScope: "u1"}, "u1", connector(&fakeSession{}, nil))
	if !errors.Is(err, session.ErrSessionLimit) {
		t.Errorf("want ErrSessionLimit, got %v", err)
	}
}

func TestIdleReclaim(t *testing.T) {
	m := session.New(session.Options{IdleTimeout: 10 * time.Millisecond, HealthInterval: 5 * time.Millisecond})
	defer m.Shutdown()
	fs := &fakeSession{}
	key := session.Key{ConnectionID: "c1", OwnerScope: "u1"}
	if _, err := m.Acquire(context.Background(), key, "u1", connector(fs, nil)); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for m.Stats().Sessions != 0 {
		select {
		case <-deadline:
			t.Fatal("idle session was not reclaimed")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if !fs.isClosed() {
		t.Error("reclaimed session was not Closed")
	}
}

func TestHealthCheckClosesDeadUpstream(t *testing.T) {
	m := session.New(session.Options{HealthInterval: 5 * time.Millisecond, IdleTimeout: time.Hour})
	defer m.Shutdown()
	fs := &fakeSession{healthErr: errors.New("upstream gone")}
	key := session.Key{ConnectionID: "c1", OwnerScope: "u1"}
	if _, err := m.Acquire(context.Background(), key, "u1", connector(fs, nil)); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for m.Stats().Sessions != 0 {
		select {
		case <-deadline:
			t.Fatal("dead upstream was not reclaimed by health check")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestShutdownClosesAll(t *testing.T) {
	m := session.New(session.Options{})
	fs := &fakeSession{}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", OwnerScope: "u1"}, "u1", connector(fs, nil)); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	m.Shutdown()
	if !fs.isClosed() {
		t.Error("shutdown did not close the session")
	}
	if s := m.Stats(); s.Sessions != 0 {
		t.Errorf("after shutdown: want 0 sessions, got %d", s.Sessions)
	}
}

func TestCloseConnectionClosesAllOwnerScopes(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()

	c1u1 := &fakeSession{}
	c1u2 := &fakeSession{}
	c2u1 := &fakeSession{}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", OwnerScope: "u1"}, "u1", connector(c1u1, nil)); err != nil {
		t.Fatalf("acquire c1 u1: %v", err)
	}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", OwnerScope: "u2"}, "u2", connector(c1u2, nil)); err != nil {
		t.Fatalf("acquire c1 u2: %v", err)
	}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c2", OwnerScope: "u1"}, "u1", connector(c2u1, nil)); err != nil {
		t.Fatalf("acquire c2 u1: %v", err)
	}

	m.CloseConnection("c1")

	if !c1u1.isClosed() || !c1u2.isClosed() {
		t.Fatal("all sessions for c1 should be closed")
	}
	if c2u1.isClosed() {
		t.Fatal("session for another connection should stay open")
	}
	if s := m.Stats(); s.Sessions != 1 {
		t.Fatalf("remaining sessions = %d, want 1", s.Sessions)
	}
}
