package recording

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
)

// recEvent is one timestamped stream event queued for the drain loop.
type recEvent struct {
	kind       byte // 'o' output, 'i' input, 'r' resize
	ts         time.Duration
	data       []byte
	cols, rows int
}

// liveRecording is the shared state between the hot stream path (which enqueues
// events without blocking) and the drain loop (which encodes them). When the
// bounded queue is full the event is dropped and the recording marked failed —
// the live stream must never block on storage.
type liveRecording struct {
	start        time.Time
	now          func() time.Time
	events       chan recEvent
	stop         chan struct{} // closed by finish; never close events (avoids send-on-closed)
	captureInput bool
	failed       atomic.Bool
	dropped      atomic.Int64
}

func (lr *liveRecording) enqueue(ev recEvent) {
	ev.ts = max(lr.now().Sub(lr.start), 0)
	select {
	case <-lr.stop:
		return
	default:
	}
	select {
	case lr.events <- ev:
	case <-lr.stop:
	default:
		lr.dropped.Add(1)
		lr.failed.Store(true)
	}
}

func (lr *liveRecording) output(p []byte) {
	lr.enqueue(recEvent{kind: 'o', data: append([]byte(nil), p...)})
}

func (lr *liveRecording) input(p []byte) {
	lr.enqueue(recEvent{kind: 'i', data: append([]byte(nil), p...)})
}

func (lr *liveRecording) resize(cols, rows int) {
	lr.enqueue(recEvent{kind: 'r', cols: cols, rows: rows})
}

// tap wraps a ClientStream and mirrors its traffic to the active recording (if
// any). client.Write carries upstream→browser output; client.Read carries
// browser→upstream input. With no active recording the tap is a passthrough.
type tap struct {
	inner plugin.ClientStream
	sess  *recSession
	live  atomic.Pointer[liveRecording]
}

func (t *tap) Read(p []byte) (int, error) {
	n, err := t.inner.Read(p)
	if n > 0 {
		frame := p[:n]
		if terminalUserInput(frame) {
			if startErr := t.startFromInteraction(); startErr != nil {
				return 0, startErr
			}
		}
		if lr := t.live.Load(); lr != nil && lr.captureInput {
			lr.input(frame)
		}
	}
	return n, err
}

func (t *tap) Write(p []byte) (int, error) {
	if lr := t.live.Load(); lr != nil {
		lr.output(p)
	}
	return t.inner.Write(p)
}

func (t *tap) Context() context.Context { return t.inner.Context() }

func (t *tap) Close() error { return t.inner.Close() }

func (t *tap) startFromInteraction() error {
	if t.sess == nil || !t.sess.shouldStartOnInteraction() {
		return nil
	}
	if err := t.sess.startOnInteraction(t.inner.Context()); err != nil {
		return fmt.Errorf("%w: required recording could not start: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func terminalUserInput(p []byte) bool {
	return len(p) > 0 && p[0] != 0
}
