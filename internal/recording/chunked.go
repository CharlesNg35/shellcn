package recording

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

// chunkedRec is a recording assembled from client-uploaded chunks (browser
// desktop capture). Chunks must arrive in order; the blob is appended as they do.
type chunkedRec struct {
	rec  models.Recording
	info StreamInfo

	mu        sync.Mutex
	nextIndex int
	size      int64
	hash      hash.Hash
	done      bool
}

// BeginChunked creates a desktop recording fed by client chunk uploads. The
// caller (HTTP handler) has already authorized the user for the connection.
func (e *Engine) BeginChunked(ctx context.Context, info StreamInfo, format plugin.RecordingFormat) (models.Recording, error) {
	capability, ok := info.Manifest.RecordingClassFor(info.StreamID)
	if !ok || capability.Class != plugin.RecordingDesktop {
		return models.Recording{}, fmt.Errorf("%w: stream is not a recordable desktop class", plugin.ErrNotSupported)
	}
	if !capability.SupportsFormat(format) {
		return models.Recording{}, fmt.Errorf("%w: unsupported desktop format %q", plugin.ErrInvalidInput, format)
	}
	if format != plugin.FormatWebMCanvas {
		return models.Recording{}, fmt.Errorf("%w: chunked desktop upload only supports %q", plugin.ErrInvalidInput, plugin.FormatWebMCanvas)
	}
	policy := plugin.RecordingPolicy(info.Connection.Recording[string(plugin.RecordingDesktop)])
	if policy == "" || policy == plugin.PolicyDisabled {
		return models.Recording{}, fmt.Errorf("%w: desktop recording is disabled", plugin.ErrForbidden)
	}

	start := e.now()
	id := uuid.NewString()
	row := models.Recording{
		ID: id, UserID: info.User.ID, Username: info.User.Username,
		ConnectionID: info.Connection.ID, ConnectionName: info.Connection.Name,
		Protocol: info.Connection.Protocol, RouteID: info.Route.ID, StreamID: info.StreamID,
		Class: string(capability.Class), Format: string(format),
		Authoritative: capability.Authoritative && format != plugin.FormatWebMCanvas,
		Status:        models.RecordingActive, Title: info.Title, StartedAt: start,
		StorageKey: StorageKey(info.Connection.ID, id, format),
		ExpiresAt:  ExpiryFor(start, info.Connection.RetentionDays, e.retention),
	}
	if err := e.store.Create(ctx, &row); err != nil {
		return models.Recording{}, err
	}

	e.mu.Lock()
	e.chunked[id] = &chunkedRec{rec: row, info: info, hash: sha256.New()}
	e.mu.Unlock()
	e.metrics.RecordingStarted()
	e.auditChunked(ctx, info, EventStart, models.AuditAllowed)
	return row, nil
}

// AppendChunk appends one ordered chunk to a chunked recording.
func (e *Engine) AppendChunk(ctx context.Context, recordingID, userID string, index int, data []byte) error {
	cr, err := e.chunkedFor(recordingID, userID)
	if err != nil {
		return err
	}
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if cr.done {
		return fmt.Errorf("%w: recording already finalized", plugin.ErrConflict)
	}
	if index != cr.nextIndex {
		return fmt.Errorf("%w: chunk %d out of order (expected %d)", plugin.ErrInvalidInput, index, cr.nextIndex)
	}
	if err := e.blobs.Append(ctx, cr.rec.StorageKey, data); err != nil {
		return err
	}
	cr.nextIndex++
	cr.size += int64(len(data))
	_, _ = cr.hash.Write(data)
	e.metrics.AddRecordingBytes(len(data))
	return nil
}

// FinalizeChunked marks a chunked recording complete.
func (e *Engine) FinalizeChunked(ctx context.Context, recordingID, userID string) (models.Recording, error) {
	cr, err := e.chunkedFor(recordingID, userID)
	if err != nil {
		return models.Recording{}, err
	}
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if !cr.done {
		cr.done = true
		end := e.now()
		cr.rec.EndedAt = &end
		cr.rec.DurationMS = end.Sub(cr.rec.StartedAt).Milliseconds()
		cr.rec.Size = cr.size
		cr.rec.Checksum = hex.EncodeToString(cr.hash.Sum(nil))
		cr.rec.Status = models.RecordingFinalized
		_ = e.store.Update(ctx, &cr.rec)
		e.metrics.RecordingFinished()
		e.auditChunked(ctx, cr.info, EventFinalize, models.AuditAllowed)
	}
	e.mu.Lock()
	delete(e.chunked, recordingID)
	e.mu.Unlock()
	return cr.rec, nil
}

// AbortChunked discards an in-progress chunked recording and its bytes.
func (e *Engine) AbortChunked(ctx context.Context, recordingID, userID string) error {
	cr, err := e.chunkedFor(recordingID, userID)
	if err != nil {
		return err
	}
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if !cr.done {
		cr.done = true
		_ = e.blobs.Delete(ctx, cr.rec.StorageKey)
		cr.rec.Status = models.RecordingDiscarded
		cr.rec.Error = "aborted"
		_ = e.store.Update(ctx, &cr.rec)
		e.metrics.RecordingFinished()
		e.auditChunked(ctx, cr.info, EventFailed, models.AuditError)
	}
	e.mu.Lock()
	delete(e.chunked, recordingID)
	e.mu.Unlock()
	return nil
}

func (e *Engine) chunkedFor(recordingID, userID string) (*chunkedRec, error) {
	e.mu.Lock()
	cr, ok := e.chunked[recordingID]
	e.mu.Unlock()
	if !ok {
		return nil, plugin.ErrNotFound
	}
	if cr.rec.UserID != userID {
		return nil, plugin.ErrForbidden
	}
	return cr, nil
}

func (e *Engine) auditChunked(ctx context.Context, info StreamInfo, event string, result models.AuditResult) {
	e.audit.Record(ctx, audit.Event{
		User: info.User, Event: event, ConnectionID: info.Connection.ID,
		RouteID: info.Route.ID, Risk: string(plugin.RiskPrivileged), Result: result,
		RemoteAddr: info.RemoteAddr,
	})
}

// ReapStaleChunked aborts chunked recordings older than maxAge, freeing partial
// blobs left by abandoned browser captures.
func (e *Engine) ReapStaleChunked(ctx context.Context, maxAge time.Duration) {
	now := e.now()
	type ref struct{ id, userID string }
	e.mu.Lock()
	var stale []ref
	for id, cr := range e.chunked {
		if now.Sub(cr.rec.StartedAt) > maxAge {
			stale = append(stale, ref{id, cr.rec.UserID})
		}
	}
	e.mu.Unlock()
	for _, r := range stale {
		_ = e.AbortChunked(ctx, r.id, r.userID)
	}
}
