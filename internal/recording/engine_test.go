package recording

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/store"
)

// --- fakes ------------------------------------------------------------------

type fakeClient struct {
	ctx     context.Context
	cancel  context.CancelFunc
	reads   chan []byte
	mu      sync.Mutex
	written [][]byte
}

func newFakeClient() *fakeClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeClient{ctx: ctx, cancel: cancel, reads: make(chan []byte, 16)}
}

func (f *fakeClient) Read(p []byte) (int, error) {
	b, ok := <-f.reads
	if !ok {
		return 0, io.EOF
	}
	return copy(p, b), nil
}

func (f *fakeClient) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, append([]byte(nil), p...))
	return len(p), nil
}

func (f *fakeClient) Close() error             { f.cancel(); return nil }
func (f *fakeClient) Context() context.Context { return f.ctx }

type fakeRecorder struct {
	w     io.Writer
	block chan struct{} // when non-nil, WriteOutput blocks until closed
	mu    sync.Mutex
	out   [][]byte
	in    [][]byte
	sizes []int
	ts    []time.Duration
}

func (r *fakeRecorder) WriteOutput(ts time.Duration, p []byte) error {
	if r.block != nil {
		<-r.block
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.out = append(r.out, append([]byte(nil), p...))
	r.ts = append(r.ts, ts)
	_, _ = r.w.Write(p)
	return nil
}

func (r *fakeRecorder) WriteInput(_ time.Duration, p []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.in = append(r.in, append([]byte(nil), p...))
	_, _ = r.w.Write(p)
	return nil
}

func (r *fakeRecorder) Resize(_ time.Duration, cols, rows int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sizes = append(r.sizes, cols, rows)
	return nil
}

func (r *fakeRecorder) Close() error { return nil }

// failBlobs makes Create fail, to exercise forced-recording denial.
type failBlobs struct{ BlobStore }

func (failBlobs) Create(context.Context, string) (io.WriteCloser, error) {
	return nil, errors.New("disk full")
}

// --- helpers ----------------------------------------------------------------

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Millisecond)
	return c.t
}

func testManifest() plugin.Manifest {
	return plugin.Manifest{
		Name: "p", Streams: []plugin.Stream{{ID: "s.sh", Kind: plugin.StreamTerminal, RouteID: "s.sh"}},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2},
			StreamIDs: []string{"s.sh"}, Authoritative: true,
		}},
	}
}

func streamInfo(policy string) StreamInfo {
	conn := models.Connection{ID: "c1", Name: "c", Protocol: "p"}
	if policy != "" {
		conn.Recording = map[string]string{"terminal": policy}
	}
	return StreamInfo{
		User: models.User{ID: "u1", Username: "u"}, Connection: conn,
		Manifest: testManifest(), Route: plugin.Route{ID: "s.sh"}, StreamID: "s.sh",
	}
}

func newEngine(t *testing.T, blobs BlobStore, rec *fakeRecorder) (*Engine, *store.Store) {
	t.Helper()
	st := store.NewMemory()
	if blobs == nil {
		var err error
		blobs, err = NewLocalBlobStore(t.TempDir())
		if err != nil {
			t.Fatalf("blobs: %v", err)
		}
	}
	clk := &clock{t: time.Unix(1700000000, 0)}
	e := NewEngine(Options{Store: st.Recordings, Blobs: blobs, Now: clk.now, BufferEvents: 64})
	e.Register(plugin.FormatAsciicastV2, func(w io.Writer, _ StartInfo) (Recorder, error) {
		rec.w = w
		return rec, nil
	})
	return e, st
}

// --- tests ------------------------------------------------------------------

func TestEngineDisabledIsPassthrough(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	client := newFakeClient()
	wrapped, finalize, err := e.Wrap(context.Background(), client, streamInfo(""))
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if wrapped != client {
		t.Error("disabled policy should return the original stream (no tap)")
	}
	finalize()
	if recs, _ := st.Recordings.List(context.Background(), store.RecordingFilter{}); len(recs) != 0 {
		t.Errorf("no recording expected, got %d", len(recs))
	}
}

func TestEngineAutoRecordsOutputWithMonotonicTimestamps(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	ctx := context.Background()

	wrapped, finalize, err := e.Wrap(ctx, newFakeClient(), streamInfo("auto"))
	if err != nil {
		t.Fatalf("wrap forced: %v", err)
	}
	for _, line := range []string{"hello\n", "world\n", "$ "} {
		if _, err := wrapped.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	finalize()

	if len(rec.out) != 3 {
		t.Fatalf("want 3 output events, got %d", len(rec.out))
	}
	for i := 1; i < len(rec.ts); i++ {
		if rec.ts[i] < rec.ts[i-1] {
			t.Errorf("timestamps not monotonic: %v", rec.ts)
		}
	}
	recs, _ := st.Recordings.List(ctx, store.RecordingFilter{})
	if len(recs) != 1 {
		t.Fatalf("want 1 recording, got %d", len(recs))
	}
	r := recs[0]
	if r.Status != models.RecordingFinalized || r.Size == 0 || r.Checksum == "" || r.EndedAt == nil {
		t.Fatalf("unexpected finalized metadata: %+v", r)
	}
	if r.Class != "terminal" || r.Format != "asciicast_v2" {
		t.Errorf("class/format: %s/%s", r.Class, r.Format)
	}
}

func TestEngineForcedFailureDeniesStream(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, failBlobs{}, rec)
	_, _, err := e.Wrap(context.Background(), newFakeClient(), streamInfo("auto"))
	if err == nil {
		t.Fatal("forced recording that cannot start must deny the stream")
	}
	if recs, _ := st.Recordings.List(context.Background(), store.RecordingFilter{}); len(recs) != 0 {
		t.Errorf("no recording row should persist on denial, got %d", len(recs))
	}
}

func TestEngineForcedFinishBeforeAttachIsSafe(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	pending, err := e.Prepare(context.Background(), streamInfo("auto"))
	if err != nil {
		t.Fatalf("prepare forced: %v", err)
	}
	pending.Finish()
	recs, _ := st.Recordings.List(context.Background(), store.RecordingFilter{})
	if len(recs) != 1 || recs[0].Status != models.RecordingFinalized {
		t.Fatalf("forced pre-attach finish should finalize safely, got %+v", recs)
	}
}

func TestEngineWebMCanvasDesktopAutoUsesChunkedPath(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	info := StreamInfo{
		User: models.User{ID: "u1"}, Connection: models.Connection{
			ID: "c1", Protocol: "vnc",
			Recording: map[string]string{"desktop": string(plugin.PolicyAuto)},
		},
		Manifest: plugin.Manifest{
			Name: "vnc",
			Streams: []plugin.Stream{{
				ID: "vnc.screen", Kind: plugin.StreamDesktop, RouteID: "vnc.screen",
			}},
			Recording: []plugin.RecordingCapability{{
				Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas},
				StreamIDs: []string{"vnc.screen"},
			}},
		},
		Route: plugin.Route{ID: "vnc.screen"}, StreamID: "vnc.screen",
	}

	wrapped, finalize, err := e.Wrap(context.Background(), newFakeClient(), info)
	if err != nil {
		t.Fatalf("webm_canvas desktop should not require server recorder: %v", err)
	}
	finalize()
	if _, ok := wrapped.(*fakeClient); !ok {
		t.Fatalf("webm_canvas desktop should not install stream tap, got %T", wrapped)
	}
	if recs, _ := st.Recordings.List(context.Background(), store.RecordingFilter{}); len(recs) != 0 {
		t.Fatalf("webm_canvas stream wrapper should not create metadata, got %+v", recs)
	}
}

func TestEngineManualStartStop(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	ctx := context.Background()
	info := streamInfo("manual")
	key := StreamKey(info.User.ID, info.Connection.ID, info.Route.ID, info.Params)

	wrapped, finalize, err := e.Wrap(ctx, newFakeClient(), info)
	if err != nil {
		t.Fatalf("wrap manual: %v", err)
	}
	// Idle: writes before Start are not recorded.
	_, _ = wrapped.Write([]byte("before\n"))
	if len(rec.out) != 0 {
		t.Fatalf("manual idle should not record, got %d events", len(rec.out))
	}

	if _, err := e.Start(ctx, key); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _ = wrapped.Write([]byte("after\n"))

	if err := e.Stop(ctx, key); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(rec.out) != 1 || string(rec.out[0]) != "after\n" {
		t.Fatalf("manual recording captured wrong data: %q", rec.out)
	}
	recs, _ := st.Recordings.List(ctx, store.RecordingFilter{})
	if len(recs) != 1 || recs[0].Status != models.RecordingFinalized {
		t.Fatalf("manual recording not finalized: %+v", recs)
	}
	finalize() // safe no-op after explicit stop
}

func TestEngineManualCanStartAgainAfterStop(t *testing.T) {
	rec := &fakeRecorder{}
	e, st := newEngine(t, nil, rec)
	ctx := context.Background()
	info := streamInfo("manual")
	key := StreamKey(info.User.ID, info.Connection.ID, info.Route.ID, info.Params)
	wrapped, finalize, err := e.Wrap(ctx, newFakeClient(), info)
	if err != nil {
		t.Fatalf("wrap manual: %v", err)
	}

	if _, err := e.Start(ctx, key); err != nil {
		t.Fatalf("first start: %v", err)
	}
	_, _ = wrapped.Write([]byte("one"))
	if err := e.Stop(ctx, key); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if _, err := e.Start(ctx, key); err != nil {
		t.Fatalf("second start: %v", err)
	}
	_, _ = wrapped.Write([]byte("two"))
	finalize()

	recs, _ := st.Recordings.List(ctx, store.RecordingFilter{})
	if len(recs) != 2 {
		t.Fatalf("want two finalized manual recordings, got %+v", recs)
	}
	for _, r := range recs {
		if r.Status != models.RecordingFinalized {
			t.Fatalf("manual recording not finalized: %+v", recs)
		}
	}
}

func TestEngineForcedCannotBeStopped(t *testing.T) {
	rec := &fakeRecorder{}
	e, _ := newEngine(t, nil, rec)
	ctx := context.Background()
	info := streamInfo("auto")
	key := StreamKey(info.User.ID, info.Connection.ID, info.Route.ID, info.Params)
	_, finalize, err := e.Wrap(ctx, newFakeClient(), info)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if err := e.Stop(ctx, key); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("forced stop: want forbidden, got %v", err)
	}
	finalize()
}

func TestEngineBackpressureDoesNotBlockStream(t *testing.T) {
	rec := &fakeRecorder{block: make(chan struct{})}
	st := store.NewMemory()
	blobs, _ := NewLocalBlobStore(t.TempDir())
	clk := &clock{t: time.Unix(1700000000, 0)}
	e := NewEngine(Options{Store: st.Recordings, Blobs: blobs, Now: clk.now, BufferEvents: 2})
	e.Register(plugin.FormatAsciicastV2, func(w io.Writer, _ StartInfo) (Recorder, error) {
		rec.w = w
		return rec, nil
	})
	ctx := context.Background()
	info := streamInfo("auto")
	wrapped, finalize, err := e.Wrap(ctx, newFakeClient(), info)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	// Storage is stuck (recorder blocks); writes must still return promptly. If
	// the tap blocked on storage this loop would hang and the test would time out.
	done := make(chan struct{})
	go func() {
		for range 50 {
			_, _ = wrapped.Write([]byte("spam"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("writes blocked on slow recorder — backpressure not bounded")
	}

	close(rec.block) // release the recorder so finalize can drain
	finalize()

	recs, _ := st.Recordings.List(ctx, store.RecordingFilter{})
	if len(recs) != 1 || recs[0].Status != models.RecordingFailed {
		t.Fatalf("overflowed recording should be marked failed: %+v", recs)
	}
}

func TestEngineConcurrentStopAndFinalize(t *testing.T) {
	rec := &fakeRecorder{}
	e, _ := newEngine(t, nil, rec)
	ctx := context.Background()
	info := streamInfo("manual")
	key := StreamKey(info.User.ID, info.Connection.ID, info.Route.ID, info.Params)
	_, finalize, err := e.Wrap(ctx, newFakeClient(), info)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if _, err := e.Start(ctx, key); err != nil {
		t.Fatalf("start: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = e.Stop(ctx, key) }()
	go func() { defer wg.Done(); finalize() }()
	wg.Wait() // -race + finishOnce prove there is no double-finalize panic
}
