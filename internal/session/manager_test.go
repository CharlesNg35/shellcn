package session_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/cluster"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// fakeSession is a controllable plugin.Session for tests.
type fakeSession struct {
	mu        sync.Mutex
	closed    bool
	healthErr error
	channel   plugin.Channel
}

func (f *fakeSession) HealthCheck(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthErr
}

func (f *fakeSession) setHealthErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthErr = err
}

func (f *fakeSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	if f.channel != nil {
		return f.channel, nil
	}
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
func (c *fakeChannel) Resize(int, int) error       { return nil }
func (c *fakeChannel) ServerInit() []byte          { return []byte("rfb") }

type basicChannel struct{ closed atomic.Bool }

func (c *basicChannel) Read([]byte) (int, error)    { return 0, io.EOF }
func (c *basicChannel) Write(p []byte) (int, error) { return len(p), nil }
func (c *basicChannel) Close() error                { c.closed.Store(true); return nil }
func (c *basicChannel) Kind() plugin.StreamKind     { return plugin.StreamLogs }

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
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}

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
	snap, ok := m.Status(key)
	if !ok {
		t.Fatal("expected status for acquired session")
	}
	if snap.State != session.StateConnected || snap.Channels != 0 || snap.UserID != "u1" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestConnectErrorNotCached(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}

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
	snap, ok := m.Status(key)
	if !ok {
		t.Fatal("failed connect should leave a status tombstone")
	}
	if snap.State != session.StateError || snap.Reason != "dial failed" {
		t.Fatalf("unexpected failure status: %+v", snap)
	}
	// A subsequent successful acquire works (entry was cleaned up).
	if _, err := m.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil)); err != nil {
		t.Errorf("retry after failure: %v", err)
	}
	if snap, ok := m.Status(key); !ok || snap.State != session.StateConnected {
		t.Fatalf("successful retry should replace failure status, ok=%v snap=%+v", ok, snap)
	}
}

func TestInitialHealthCheckFailureNotCached(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	fs := &fakeSession{healthErr: errors.New("unhealthy")}

	_, err := m.Acquire(context.Background(), key, "u1", connector(fs, nil))
	if !errors.Is(err, fs.healthErr) {
		t.Fatalf("want health error, got %v", err)
	}
	if !fs.isClosed() {
		t.Fatal("session should close after initial health failure")
	}
	if got := m.Stats().Sessions; got != 0 {
		t.Fatalf("failed health session should not stay live, got %d", got)
	}
	snap, ok := m.Status(key)
	if !ok || snap.State != session.StateError || snap.Reason != "unhealthy" {
		t.Fatalf("unexpected failure status, ok=%v snap=%+v", ok, snap)
	}
}

func TestConcurrentAcquireCreatesOneSession(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	fs := &fakeSession{}

	var hits int32
	start := make(chan struct{})
	connect := func(context.Context) (plugin.Session, error) {
		<-start
		atomic.AddInt32(&hits, 1)
		return fs, nil
	}

	const callers = 8
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	handles := make(chan *session.Handle, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := m.Acquire(context.Background(), key, "u1", connect)
			if err != nil {
				errs <- err
				return
			}
			handles <- h
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(handles)

	for err := range errs {
		t.Fatalf("acquire: %v", err)
	}
	for h := range handles {
		if h.Session() != fs {
			t.Fatal("all callers should receive the same plugin session")
		}
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("connect ran %d times, want 1", hits)
	}
}

func TestChannelTrackingAndLimit(t *testing.T) {
	m := session.New(session.Options{MaxChannelsPerSession: 2})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
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
	if snap, ok := m.Status(key); !ok || snap.Channels != 2 {
		t.Fatalf("status should report 2 open channels, got ok=%v snap=%+v", ok, snap)
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

func TestTrackedChannelPreservesOptionalCapabilities(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	h, _ := m.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil))

	ch, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamTerminal})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := ch.(interface{ ServerInit() []byte }).ServerInit(); string(got) != "rfb" {
		t.Fatalf("server init = %q", got)
	}
	if err := ch.(interface{ Resize(int, int) error }).Resize(120, 40); err != nil {
		t.Fatalf("resize: %v", err)
	}
}

func TestTrackedChannelDoesNotInventOptionalCapabilities(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	h, _ := m.Acquire(context.Background(), key, "u1", connector(&fakeSession{channel: &basicChannel{}}, nil))

	ch, err := h.OpenChannel(context.Background(), plugin.ChannelRequest{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, ok := ch.(interface{ ServerInit() []byte }); ok {
		t.Fatal("tracked channel invented ServerInit")
	}
	if _, ok := ch.(interface{ Resize(int, int) error }); ok {
		t.Fatal("tracked channel invented Resize")
	}
}

func TestActiveStreamPreventsIdleReclaim(t *testing.T) {
	m := session.New(session.Options{IdleTimeout: 10 * time.Millisecond, HealthInterval: 5 * time.Millisecond})
	defer m.Shutdown()
	fs := &fakeSession{}
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	h, err := m.Acquire(context.Background(), key, "u1", connector(fs, nil))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release := h.TrackStream()
	defer release()

	time.Sleep(40 * time.Millisecond)
	if got := m.Stats().Sessions; got != 1 {
		t.Fatalf("active stream session was reclaimed: sessions=%d", got)
	}
	if snap, ok := m.Status(key); !ok || snap.Streams != 1 {
		t.Fatalf("status should report active stream, ok=%v snap=%+v", ok, snap)
	}

	release()
	deadline := time.After(2 * time.Second)
	for m.Stats().Sessions != 0 {
		select {
		case <-deadline:
			t.Fatal("idle session was not reclaimed after stream release")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if !fs.isClosed() {
		t.Fatal("session should close after stream release and idle timeout")
	}
}

func TestPerUserSessionLimit(t *testing.T) {
	m := session.New(session.Options{MaxSessionsPerUser: 1})
	defer m.Shutdown()
	_, err := m.Acquire(context.Background(), session.Key{ConnectionID: "a", ActorScope: "u1"}, "u1", connector(&fakeSession{}, nil))
	if err != nil {
		t.Fatalf("first session: %v", err)
	}
	_, err = m.Acquire(context.Background(), session.Key{ConnectionID: "b", ActorScope: "u1"}, "u1", connector(&fakeSession{}, nil))
	if !errors.Is(err, session.ErrSessionLimit) {
		t.Errorf("want ErrSessionLimit, got %v", err)
	}
}

func TestIdleReclaim(t *testing.T) {
	m := session.New(session.Options{IdleTimeout: 10 * time.Millisecond, HealthInterval: 5 * time.Millisecond})
	defer m.Shutdown()
	fs := &fakeSession{}
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
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
	fs := &fakeSession{}
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	if _, err := m.Acquire(context.Background(), key, "u1", connector(fs, nil)); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	fs.setHealthErr(errors.New("upstream gone"))

	deadline := time.After(2 * time.Second)
	for m.Stats().Sessions != 0 {
		select {
		case <-deadline:
			t.Fatal("dead upstream was not reclaimed by health check")
		case <-time.After(5 * time.Millisecond):
		}
	}
	snap, ok := m.Status(key)
	if !ok {
		t.Fatal("dead upstream should leave a failure status")
	}
	if snap.State != session.StateError || snap.Reason != "upstream gone" || snap.LastHealthCheck.IsZero() {
		t.Fatalf("unexpected failure status: %+v", snap)
	}
	if !fs.isClosed() {
		t.Fatal("dead upstream session should be closed")
	}
}

func TestFailureStatusExpires(t *testing.T) {
	m := session.New(session.Options{FailureRetention: 15 * time.Millisecond})
	defer m.Shutdown()
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	_, _ = m.Acquire(context.Background(), key, "u1", func(context.Context) (plugin.Session, error) {
		return nil, errors.New("dial failed")
	})
	if _, ok := m.Status(key); !ok {
		t.Fatal("expected failure status")
	}
	time.Sleep(30 * time.Millisecond)
	if snap, ok := m.Status(key); ok {
		t.Fatalf("failure status should expire, got %+v", snap)
	}
}

func TestShutdownClosesAll(t *testing.T) {
	m := session.New(session.Options{})
	fs := &fakeSession{}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", ActorScope: "u1"}, "u1", connector(fs, nil)); err != nil {
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

func TestCloseConnectionClosesAllActorScopes(t *testing.T) {
	m := session.New(session.Options{})
	defer m.Shutdown()

	c1u1 := &fakeSession{}
	c1u2 := &fakeSession{}
	c2u1 := &fakeSession{}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", ActorScope: "u1"}, "u1", connector(c1u1, nil)); err != nil {
		t.Fatalf("acquire c1 u1: %v", err)
	}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c1", ActorScope: "u2"}, "u2", connector(c1u2, nil)); err != nil {
		t.Fatalf("acquire c1 u2: %v", err)
	}
	if _, err := m.Acquire(context.Background(), session.Key{ConnectionID: "c2", ActorScope: "u1"}, "u1", connector(c2u1, nil)); err != nil {
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

func TestAcquireClaimsExclusiveOwner(t *testing.T) {
	owners := cluster.NewStoreOwnerRegistry(store.NewMemory().ClusterOwners)
	key := session.Key{ConnectionID: "c1", ActorScope: "u1"}
	first := session.New(session.Options{
		OwnerRegistry: owners,
		Instance:      cluster.NewInstanceRef("a", "http://a"),
	})
	defer first.Shutdown()
	second := session.New(session.Options{
		OwnerRegistry: owners,
		Instance:      cluster.NewInstanceRef("b", "http://b"),
	})
	defer second.Shutdown()

	if _, err := first.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil)); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := second.Acquire(context.Background(), key, "u1", connector(&fakeSession{}, nil)); !errors.Is(err, cluster.ErrOwnedElsewhere) {
		t.Fatalf("second acquire: want ErrOwnedElsewhere, got %v", err)
	}
}
