// Package audit records an append-only log of every authorized (and denied)
// operation. Params arrive already redacted; the writer never mutates audit
// rows after insert.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

// Operation sources recorded on every audit event.
const (
	SourceHTTP = "http"
	SourceAI   = "ai"
)

// Event is one auditable operation, assembled by the route wrapper. Params must
// already have secret values redacted before they reach Record.
type Event struct {
	User         models.User
	Event        string // route AuditEvent
	ConnectionID string
	RouteID      string
	Risk         string
	Result       models.AuditResult
	Params       map[string]string
	Err          error
	RemoteAddr   string
	// Source is "http" or "ai"; TurnID correlates AI tool calls to a turn. Both
	// fall back to the request context when left empty (see WithSource).
	Source string
	TurnID string
}

// Sink receives audit events. The route wrapper depends on this interface; the
// store-backed writer and a noop sink both implement it.
type Sink interface {
	Record(ctx context.Context, ev Event)
}

type ctxKey int

const (
	remoteAddrKey ctxKey = iota
	sourceKey
)

// WithRemoteAddr stashes the request's client address on the context so every
// audit event recorded during the request inherits it without each call site
// having to thread it through.
func WithRemoteAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, remoteAddrKey, addr)
}

func remoteAddrFrom(ctx context.Context) string {
	addr, _ := ctx.Value(remoteAddrKey).(string)
	return addr
}

type origin struct {
	source string
	turnID string
}

// WithSource marks every audit event recorded during ctx with how the operation
// was initiated ("http" or "ai") and, for AI, the conversation/turn id.
func WithSource(ctx context.Context, source, turnID string) context.Context {
	return context.WithValue(ctx, sourceKey, origin{source: source, turnID: turnID})
}

func sourceFrom(ctx context.Context) (string, string) {
	o, _ := ctx.Value(sourceKey).(origin)
	return o.source, o.turnID
}

// Writer persists events to the append-only AuditStore.
type Writer struct {
	store store.AuditStore
	now   func() time.Time
}

func NewWriter(s store.AuditStore) *Writer {
	return &Writer{store: s, now: time.Now}
}

// Record appends one audit entry. Append failures are intentionally swallowed
// here (audit must never break the request path); the store logs its own errors.
func (w *Writer) Record(ctx context.Context, ev Event) {
	addr := ev.RemoteAddr
	if addr == "" {
		addr = remoteAddrFrom(ctx)
	}
	source, turnID := ev.Source, ev.TurnID
	if source == "" {
		source, turnID = sourceFrom(ctx)
	}
	entry := &models.AuditEntry{
		ID:           uuid.NewString(),
		Time:         w.now(),
		UserID:       ev.User.ID,
		Username:     ev.User.Username,
		Event:        ev.Event,
		ConnectionID: ev.ConnectionID,
		RouteID:      ev.RouteID,
		Risk:         ev.Risk,
		Result:       ev.Result,
		Params:       ev.Params,
		RemoteAddr:   addr,
		Source:       source,
		TurnID:       turnID,
	}
	if ev.Err != nil {
		entry.Error = ev.Err.Error()
	}
	_ = w.store.Append(ctx, entry)
}

// Noop discards events — used by the route wrapper until the real writer is wired.
type Noop struct{}

// Record does nothing.
func (Noop) Record(context.Context, Event) {}
