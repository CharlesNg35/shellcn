package recording

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Audit event names for recording lifecycle, kept separate from the route audit.
const (
	EventStart    = "recording.start"
	EventFinalize = "recording.finalize"
	EventFailed   = "recording.failed"
	EventRead     = "recording.read"
	EventDelete   = "recording.delete"
)

// defaultBufferEvents bounds the per-recording event queue. A full queue marks
// the recording failed rather than blocking the live stream.
const defaultBufferEvents = 1024

// Options configures an Engine.
type Options struct {
	Store                store.RecordingStore
	Blobs                BlobStore
	Audit                audit.Sink
	Metrics              Metrics
	DefaultRetentionDays int
	BufferEvents         int
	Now                  func() time.Time
}

// Engine decides whether a stream is recorded and owns recording lifecycle.
type Engine struct {
	store     store.RecordingStore
	blobs     BlobStore
	audit     audit.Sink
	metrics   Metrics
	now       func() time.Time
	bufEvents int
	retention int
	factories map[plugin.RecordingFormat]RecorderFactory

	mu      sync.Mutex
	active  map[string]*recSession // streamed (tap) recordings, keyed by StreamKey
	chunked map[string]*chunkedRec // client-uploaded recordings, keyed by recording id
}

// NewEngine builds an Engine. Register a RecorderFactory per format before use.
func NewEngine(opts Options) *Engine {
	e := &Engine{
		store:     opts.Store,
		blobs:     opts.Blobs,
		audit:     opts.Audit,
		metrics:   opts.Metrics,
		now:       opts.Now,
		bufEvents: opts.BufferEvents,
		retention: opts.DefaultRetentionDays,
		factories: map[plugin.RecordingFormat]RecorderFactory{},
		active:    map[string]*recSession{},
		chunked:   map[string]*chunkedRec{},
	}
	if e.now == nil {
		e.now = time.Now
	}
	if e.metrics == nil {
		e.metrics = noopMetrics{}
	}
	if e.audit == nil {
		e.audit = audit.Noop{}
	}
	if e.bufEvents <= 0 {
		e.bufEvents = defaultBufferEvents
	}
	return e
}

// Register associates a recorder factory with a format.
func (e *Engine) Register(format plugin.RecordingFormat, f RecorderFactory) {
	e.factories[format] = f
}

// StreamInfo describes the stream the wrapper is about to serve.
type StreamInfo struct {
	User       models.User
	Connection models.Connection
	Manifest   plugin.Manifest
	Route      plugin.Route
	StreamID   string
	Params     map[string]string
	Cols       int
	Rows       int
	Title      string
	RemoteAddr string
}

// StreamKey locates a live stream's recording tap.
func StreamKey(userID, connectionID, routeID string, params map[string]string) string {
	return userID + "|" + connectionID + "|" + routeID + "|" + canonicalParams(params)
}

func canonicalParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
		b.WriteByte(';')
	}
	return b.String()
}

// Pending is the recording decision for a stream.
type Pending struct {
	engine *Engine
	sess   *recSession
}

// Prepare decides recording from plugin capability and connection policy.
func (e *Engine) Prepare(ctx context.Context, info StreamInfo) (*Pending, error) {
	if e == nil {
		return &Pending{}, nil
	}
	capability, ok := info.Manifest.RecordingClassFor(info.StreamID)
	if !ok {
		return &Pending{}, nil
	}
	policy := plugin.RecordingPolicy(info.Connection.Recording[string(capability.Class)])
	if policy == "" || policy == plugin.PolicyDisabled {
		return &Pending{}, nil
	}
	sess := &recSession{
		engine:     e,
		key:        StreamKey(info.User.ID, info.Connection.ID, info.Route.ID, info.Params),
		info:       info,
		capability: capability,
		forced:     policy == plugin.PolicyAuto,
	}
	if sess.forced && capability.Class == plugin.RecordingDesktop && capability.DefaultFormat() == plugin.FormatWebMCanvas {
		return nil, fmt.Errorf("%w: required desktop recording cannot be enforced by browser capture", plugin.ErrUnavailable)
	}
	if sess.forced && capability.Class != plugin.RecordingTerminal {
		if err := e.startSession(ctx, sess); err != nil {
			return nil, fmt.Errorf("%w: required recording could not start: %v", plugin.ErrUnavailable, err)
		}
	}
	return &Pending{engine: e, sess: sess}, nil
}

// Recording reports whether this Pending will (or already does) record.
func (p *Pending) Recording() bool { return p != nil && p.sess != nil }

// Attach wraps the live client stream with the recording tap.
func (p *Pending) Attach(client plugin.ClientStream) plugin.ClientStream {
	if p == nil || p.sess == nil {
		return client
	}
	t := &tap{inner: client, sess: p.sess}
	p.sess.tap = t
	if p.sess.live.Load() {
		t.live.Store(p.sess.lr) // forced recording already active
	}
	p.engine.mu.Lock()
	p.engine.active[p.sess.key] = p.sess
	p.engine.mu.Unlock()
	return t
}

// Finish ends the recording (if any) when the stream closes.
func (p *Pending) Finish() {
	if p == nil || p.sess == nil {
		return
	}
	p.engine.mu.Lock()
	delete(p.engine.active, p.sess.key)
	p.engine.mu.Unlock()
	p.sess.finish(models.RecordingFinalized)
}

// Wrap is a convenience that prepares, attaches, and returns a finalize func.
func (e *Engine) Wrap(ctx context.Context, client plugin.ClientStream, info StreamInfo) (plugin.ClientStream, func(), error) {
	p, err := e.Prepare(ctx, info)
	if err != nil {
		return nil, nil, err
	}
	return p.Attach(client), p.Finish, nil
}

// Start activates recording for a manual (idle) stream identified by key. It is a
// no-op if the stream is already recording.
func (e *Engine) Start(ctx context.Context, key string) (models.Recording, error) {
	e.mu.Lock()
	sess, ok := e.active[key]
	e.mu.Unlock()
	if !ok {
		return models.Recording{}, plugin.ErrNotFound
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.live.Load() {
		return *sess.rec, nil
	}
	if err := e.startSessionLocked(ctx, sess); err != nil {
		return models.Recording{}, err
	}
	return *sess.rec, nil
}

// Stop finalizes a manual recording identified by key. Forced recordings cannot
// be stopped while their stream is live.
func (e *Engine) Stop(_ context.Context, key string) error {
	e.mu.Lock()
	sess, ok := e.active[key]
	e.mu.Unlock()
	if !ok {
		return plugin.ErrNotFound
	}
	if sess.forced {
		return fmt.Errorf("%w: forced recording cannot be stopped", plugin.ErrForbidden)
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if !sess.live.Load() {
		return nil
	}
	sess.finishLocked(models.RecordingFinalized)
	return nil
}

// startSession creates the recording row + blob + recorder and begins draining.
func (e *Engine) startSession(ctx context.Context, sess *recSession) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return e.startSessionLocked(ctx, sess)
}

func (e *Engine) startSessionLocked(ctx context.Context, sess *recSession) error {
	if sess.live.Load() {
		return nil
	}
	format := sess.capability.DefaultFormat()
	factory, ok := e.factories[format]
	if !ok {
		return fmt.Errorf("no recorder for format %q", format)
	}
	start := e.now()
	id := uuid.NewString()
	storageKey := StorageKey(sess.info.Connection.ID, id, format)

	w, err := e.blobs.Create(ctx, storageKey)
	if err != nil {
		return err
	}
	counter := newCountingWriter(w)
	rec, err := factory(counter, StartInfo{
		Title: sess.info.Title, Cols: sess.info.Cols, Rows: sess.info.Rows,
		Start: start, Format: format,
	})
	if err != nil {
		_ = w.Close()
		_ = e.blobs.Delete(ctx, storageKey)
		return err
	}

	row := &models.Recording{
		ID: id, UserID: sess.info.User.ID, Username: sess.info.User.Username,
		ConnectionID: sess.info.Connection.ID, ConnectionName: sess.info.Connection.Name,
		Protocol: sess.info.Connection.Protocol, RouteID: sess.info.Route.ID, StreamID: sess.info.StreamID,
		Class: string(sess.capability.Class), Format: string(format), Authoritative: sess.capability.Authoritative,
		Status: models.RecordingActive, Title: sess.info.Title, StartedAt: start,
		StorageKey: storageKey, ExpiresAt: ExpiryFor(start, sess.info.Connection.RetentionDays, e.retention),
	}
	if err := e.store.Create(ctx, row); err != nil {
		_ = rec.Close()
		_ = w.Close()
		_ = e.blobs.Delete(ctx, storageKey)
		return err
	}

	lr := &liveRecording{
		start: start, now: e.now, events: make(chan recEvent, e.bufEvents),
		stop:         make(chan struct{}),
		captureInput: false, // input (`i`) capture is sensitive and off by default
	}
	sess.ctx = context.WithoutCancel(ctx)
	sess.rec = row
	sess.recorder = rec
	sess.blob = w
	sess.counter = counter
	sess.lr = lr
	sess.drainDone = make(chan struct{})
	sess.live.Store(true)

	e.metrics.RecordingStarted()
	go sess.drain(e.metrics)
	// On manual Start the tap already exists; for forced Prepare it is linked later
	// by Attach once the live client stream is available.
	if sess.tap != nil {
		sess.tap.live.Store(lr)
	}
	e.auditRecording(sess.ctx, sess, EventStart, models.AuditAllowed, nil)
	return nil
}

func (e *Engine) auditRecording(ctx context.Context, sess *recSession, event string, result models.AuditResult, err error) {
	e.audit.Record(ctx, audit.Event{
		User: sess.info.User, Event: event, ConnectionID: sess.info.Connection.ID,
		RouteID: sess.info.Route.ID, Risk: string(plugin.RiskPrivileged), Result: result,
		RemoteAddr: sess.info.RemoteAddr, Err: err,
	})
}
