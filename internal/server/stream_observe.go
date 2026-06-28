package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	defaultStreamSampleDuration = 1200 * time.Millisecond
	maxStreamSampleDuration     = 5 * time.Second
	defaultStreamSampleBytes    = 16 << 10
	maxStreamSampleBytes        = 64 << 10
	defaultStreamSampleEvents   = 100
	maxStreamSampleEvents       = 500
	streamSampleCloseGrace      = 500 * time.Millisecond
)

func (s *Server) InvokeStream(ctx context.Context, user models.User, connID, routeID string, params map[string]string, opts engine.StreamSampleOptions) (any, error) {
	res, err := s.resolveRoute(ctx, user, connID, routeID, params)
	if err != nil {
		if res.route.ID != "" {
			s.auditEvent(ctx, res, models.AuditDenied, err)
			s.incAuthzFailure(err)
		}
		return nil, err
	}
	if !res.route.IsStream() {
		return nil, plugin.ErrNotSupported
	}
	stream, ok := res.plg.Manifest().StreamByRoute(res.route.ID)
	if !ok || res.route.Risk != plugin.RiskSafe {
		err := fmt.Errorf("%w: stream route is not observable", plugin.ErrForbidden)
		s.auditEvent(ctx, res, models.AuditDenied, err)
		return nil, err
	}

	opts = normalizeStreamSampleOptions(opts)
	ctx, cancelRoute := routeContext(ctx, res.route)
	defer cancelRoute()
	auditCtx := context.WithoutCancel(ctx)
	sampleCtx, cancelSample := context.WithTimeout(ctx, opts.Duration)
	defer cancelSample()

	handle, err := s.acquireSession(sampleCtx, res)
	if err != nil {
		s.auditEvent(auditCtx, res, models.AuditError, err)
		return nil, err
	}
	releaseStream := handle.TrackStream()
	defer releaseStream()

	body, err := streamValidationBody(res.route.Input, res.params)
	if err != nil {
		s.auditEvent(auditCtx, res, models.AuditError, err)
		return nil, err
	}
	rc := plugin.NewRequestContext(sampleCtx, toPluginUser(user), handle, res.params, streamQuery(res.params), body).
		WithStorage(s.pluginStorage(res)).
		WithAuditHook(func(ctx context.Context, result plugin.AuditResult, params map[string]string, err error) {
			s.auditEventParams(ctx, res, models.AuditResult(result), params, err)
		}).
		WithProxyPrefix(connProxyPrefix(res.conn.ID))
	if err := rc.ValidateSchema(res.route.Input); err != nil {
		s.auditEvent(auditCtx, res, models.AuditError, err)
		return nil, err
	}

	observer := newStreamObserver(sampleCtx, cancelSample, res.route.ID, stream, opts)
	done := make(chan error, 1)
	go func() {
		done <- res.route.Stream(rc, observer)
	}()

	var streamErr error
	select {
	case streamErr = <-done:
	case <-sampleCtx.Done():
		_ = observer.Close()
		select {
		case streamErr = <-done:
		case <-time.After(streamSampleCloseGrace):
			streamErr = nil
		}
	}
	if streamErr != nil && !benignStreamSampleError(streamErr) {
		s.auditEvent(auditCtx, res, models.AuditError, streamErr)
		return nil, streamErr
	}
	s.auditEvent(auditCtx, res, models.AuditAllowed, nil)
	return observer.Sample(), nil
}

func normalizeStreamSampleOptions(opts engine.StreamSampleOptions) engine.StreamSampleOptions {
	if opts.Duration <= 0 {
		opts.Duration = defaultStreamSampleDuration
	}
	if opts.Duration > maxStreamSampleDuration {
		opts.Duration = maxStreamSampleDuration
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = defaultStreamSampleBytes
	}
	if opts.MaxBytes > maxStreamSampleBytes {
		opts.MaxBytes = maxStreamSampleBytes
	}
	if opts.MaxEvents <= 0 {
		opts.MaxEvents = defaultStreamSampleEvents
	}
	if opts.MaxEvents > maxStreamSampleEvents {
		opts.MaxEvents = maxStreamSampleEvents
	}
	return opts
}

func streamQuery(params map[string]string) url.Values {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return q
}

func streamValidationBody(schema *plugin.Schema, params map[string]string) ([]byte, error) {
	if schema == nil {
		return nil, nil
	}
	values := map[string]any{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			raw, ok := params[field.Key]
			if !ok {
				continue
			}
			values[field.Key] = streamFieldValue(field, raw)
		}
	}
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func streamFieldValue(field plugin.Field, raw string) any {
	switch field.Type {
	case plugin.FieldNumber, plugin.FieldStepper, plugin.FieldSlider:
		if n, err := strconv.ParseFloat(raw, 64); err == nil {
			return n
		}
	case plugin.FieldToggle:
		if b, err := strconv.ParseBool(raw); err == nil {
			return b
		}
	}
	return raw
}

func benignStreamSampleError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		strings.Contains(err.Error(), "use of closed network connection")
}

type streamObserver struct {
	ctx       context.Context
	cancel    context.CancelFunc
	routeID   string
	stream    plugin.Stream
	startedAt time.Time
	maxBytes  int
	maxEvents int

	mu        sync.Mutex
	buf       []byte
	bytes     int
	events    int
	truncated bool
	closed    bool
}

func newStreamObserver(ctx context.Context, cancel context.CancelFunc, routeID string, stream plugin.Stream, opts engine.StreamSampleOptions) *streamObserver {
	return &streamObserver{
		ctx: ctx, cancel: cancel, routeID: routeID, stream: stream, startedAt: time.Now(),
		maxBytes: opts.MaxBytes, maxEvents: opts.MaxEvents,
	}
}

func (o *streamObserver) Context() context.Context { return o.ctx }

func (o *streamObserver) Read([]byte) (int, error) {
	<-o.ctx.Done()
	return 0, io.EOF
}

func (o *streamObserver) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return 0, io.ErrClosedPipe
	}
	select {
	case <-o.ctx.Done():
		o.closed = true
		return 0, io.ErrClosedPipe
	default:
	}
	if len(p) > 0 {
		o.events++
		remaining := o.maxBytes - len(o.buf)
		if remaining > 0 {
			if len(p) <= remaining {
				o.buf = append(o.buf, p...)
			} else {
				o.buf = append(o.buf, p[:remaining]...)
				o.truncated = true
			}
		} else {
			o.truncated = true
		}
		o.bytes += len(p)
		if o.events >= o.maxEvents || len(o.buf) >= o.maxBytes {
			o.truncated = o.truncated || o.events >= o.maxEvents || o.bytes > len(o.buf)
			o.cancel()
		}
	}
	return len(p), nil
}

func (o *streamObserver) Close() error {
	o.mu.Lock()
	o.closed = true
	o.mu.Unlock()
	o.cancel()
	return nil
}

func (o *streamObserver) Sample() engine.StreamSample {
	o.mu.Lock()
	defer o.mu.Unlock()
	return engine.StreamSample{
		RouteID:    o.routeID,
		StreamID:   o.stream.ID,
		Kind:       string(o.stream.Kind),
		DurationMS: int(time.Since(o.startedAt) / time.Millisecond),
		Bytes:      o.bytes,
		Events:     o.events,
		Truncated:  o.truncated,
		Data:       string(o.buf),
	}
}
