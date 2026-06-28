package recording

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const preStartTerminalOutputLimit = 256 << 10

// recSession ties one stream's tap to its recording: metadata row, blob writer,
// format recorder, and the drain goroutine. A session may be idle (manual policy
// before Start) — only `live` sessions own a recorder + drain loop.
type recSession struct {
	engine     *Engine
	key        string
	info       StreamInfo
	capability plugin.RecordingCapability
	forced     bool
	tap        *tap

	ctx       context.Context
	rec       *models.Recording
	recorder  Recorder
	blob      io.WriteCloser
	counter   *countingWriter
	lr        *liveRecording
	drainDone chan struct{}

	preStartOutput      [][]byte
	preStartOutputBytes int

	mu   sync.Mutex
	live atomic.Bool
}

// drain encodes queued events into the recorder until the session stops, then
// flushes any buffered events. It reports bytes written to metrics as it goes.
func (s *recSession) drain(metrics Metrics) {
	defer close(s.drainDone)
	var reported int64
	report := func() {
		if n := s.counter.n - reported; n > 0 {
			metrics.AddRecordingBytes(int(n))
			reported = s.counter.n
		}
	}
	write := func(ev recEvent) {
		var err error
		switch ev.kind {
		case 'o':
			err = s.recorder.WriteOutput(ev.ts, ev.data)
		case 'i':
			err = s.recorder.WriteInput(ev.ts, ev.data)
		case 'r':
			err = s.recorder.Resize(ev.ts, ev.cols, ev.rows)
		}
		if err != nil {
			s.lr.failed.Store(true)
		}
		report()
	}
	for {
		select {
		case ev := <-s.lr.events:
			write(ev)
		case <-s.lr.stop:
			for {
				select {
				case ev := <-s.lr.events:
					write(ev)
				default:
					report()
					return
				}
			}
		}
	}
}

// finish stops draining, closes the recorder + blob, and persists the final
// metadata exactly once. A failed capture is recorded as RecordingFailed.
func (s *recSession) finish(status models.RecordingStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishLocked(status)
}

func (s *recSession) finishLocked(status models.RecordingStatus) {
	if !s.live.Load() {
		return
	}
	s.live.Store(false)
	if s.tap != nil {
		s.tap.live.Store(nil)
	}
	close(s.lr.stop)
	<-s.drainDone

	if err := s.recorder.Close(); err != nil {
		s.lr.failed.Store(true)
	}
	if err := s.blob.Close(); err != nil {
		s.lr.failed.Store(true)
	}

	end := s.engine.now()
	s.rec.EndedAt = &end
	s.rec.DurationMS = end.Sub(s.rec.StartedAt).Milliseconds()
	s.rec.Size = s.counter.n
	s.rec.Checksum = s.counter.checksum()
	event := EventFinalize
	if s.lr.failed.Load() {
		s.rec.Status = models.RecordingFailed
		s.rec.Error = "capture incomplete (recorder error or buffer overflow)"
		event = EventFailed
	} else {
		s.rec.Status = status
	}
	updateErr := s.engine.store.Update(s.ctx, s.rec)
	s.engine.metrics.RecordingFinished()
	if updateErr != nil {
		s.engine.metrics.RecordingFailed()
		s.engine.auditRecording(s.ctx, s, EventFailed, models.AuditError, updateErr)
	} else if s.rec.Status == models.RecordingFailed {
		s.engine.metrics.RecordingFailed()
		s.engine.auditRecording(s.ctx, s, event, models.AuditError, nil)
	} else {
		s.engine.auditRecording(s.ctx, s, event, models.AuditAllowed, nil)
	}
}

func (s *recSession) shouldStartOnInteraction() bool {
	return s.forced && s.capability.Class == plugin.RecordingTerminal && !s.live.Load()
}

func (s *recSession) shouldBufferPreStartOutput() bool {
	return s.forced && s.capability.Class == plugin.RecordingTerminal && !s.live.Load()
}

func (s *recSession) recordOrBufferPreStartOutput(p []byte) {
	if len(p) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.live.Load() {
		if s.lr != nil {
			s.lr.output(p)
		}
		return
	}
	if !s.shouldBufferPreStartOutput() {
		return
	}
	chunk := append([]byte(nil), p...)
	if len(chunk) > preStartTerminalOutputLimit {
		chunk = chunk[len(chunk)-preStartTerminalOutputLimit:]
		s.preStartOutput = s.preStartOutput[:0]
		s.preStartOutputBytes = 0
	}
	s.preStartOutput = append(s.preStartOutput, chunk)
	s.preStartOutputBytes += len(chunk)
	for s.preStartOutputBytes > preStartTerminalOutputLimit && len(s.preStartOutput) > 0 {
		s.preStartOutputBytes -= len(s.preStartOutput[0])
		copy(s.preStartOutput, s.preStartOutput[1:])
		s.preStartOutput[len(s.preStartOutput)-1] = nil
		s.preStartOutput = s.preStartOutput[:len(s.preStartOutput)-1]
	}
}

func (s *recSession) flushPreStartOutputLocked() {
	if s.lr == nil || len(s.preStartOutput) == 0 {
		return
	}
	for _, chunk := range s.preStartOutput {
		s.lr.output(chunk)
	}
	for i := range s.preStartOutput {
		s.preStartOutput[i] = nil
	}
	s.preStartOutput = nil
	s.preStartOutputBytes = 0
}

func (s *recSession) startOnInteraction(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.shouldStartOnInteraction() {
		return nil
	}
	if err := s.engine.startSessionLocked(ctx, s); err != nil {
		return err
	}
	s.flushPreStartOutputLocked()
	return nil
}

// Resize records a terminal resize for the live recording identified by key (the
// terminal resize control channel calls this). No-op when not recording.
func (e *Engine) Resize(key string, cols, rows int) {
	e.mu.Lock()
	sess, ok := e.active[key]
	e.mu.Unlock()
	if !ok || !sess.live.Load() {
		return
	}
	sess.lr.resize(cols, rows)
}
