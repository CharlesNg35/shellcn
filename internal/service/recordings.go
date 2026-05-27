package service

import (
	"context"
	"io"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/store"
)

// RecordingService is the control-plane read/lifecycle side of recordings:
// authorized listing, retrieval, deletion, blob content access, and retention
// cleanup. Read scope is role-aware — admins see everything, others see their
// own recordings only.
type RecordingService struct {
	recs  store.RecordingStore
	blobs recording.BlobStore
}

// NewRecordingService wires the dependencies.
func NewRecordingService(recs store.RecordingStore, blobs recording.BlobStore) *RecordingService {
	return &RecordingService{recs: recs, blobs: blobs}
}

// Create persists initial recording metadata.
func (s *RecordingService) Create(ctx context.Context, r *models.Recording) error {
	return s.recs.Create(ctx, r)
}

// canView reports whether actor may see a recording. Admins may view all
// recordings; non-admins may only view recordings they created.
func (s *RecordingService) canView(ctx context.Context, actor models.User, r models.Recording) bool {
	_ = ctx
	return actor.HasRole(models.RoleAdmin) || r.UserID == actor.ID
}

// List returns recordings the actor may see. Admins get the unfiltered query
// (including the optional user filter for per-user drill-down); everyone else is
// always scoped to their own recordings.
func (s *RecordingService) List(ctx context.Context, actor models.User, f store.RecordingFilter) ([]models.Recording, error) {
	if actor.HasRole(models.RoleAdmin) {
		return s.recs.List(ctx, f)
	}

	f.UserID = actor.ID
	return s.recs.List(ctx, f)
}

// Get returns one recording if the actor may see it.
func (s *RecordingService) Get(ctx context.Context, actor models.User, id string) (models.Recording, error) {
	r, err := s.recs.Get(ctx, id)
	if err != nil {
		return models.Recording{}, err
	}
	if !s.canView(ctx, actor, r) {
		return models.Recording{}, plugin.ErrForbidden
	}
	return r, nil
}

// Content opens the recording's blob for a finalized recording the actor may see.
func (s *RecordingService) Content(ctx context.Context, actor models.User, id string) (io.ReadCloser, models.Recording, error) {
	r, err := s.Get(ctx, actor, id)
	if err != nil {
		return nil, models.Recording{}, err
	}
	if r.Status != models.RecordingFinalized {
		return nil, r, plugin.ErrUnavailable
	}
	rc, err := s.blobs.Open(ctx, r.StorageKey)
	if err != nil {
		return nil, r, err
	}
	return rc, r, nil
}

// Delete removes a recording's blob and metadata if the actor may manage it.
func (s *RecordingService) Delete(ctx context.Context, actor models.User, id string) (models.Recording, error) {
	r, err := s.recs.Get(ctx, id)
	if err != nil {
		return models.Recording{}, err
	}
	if !s.canView(ctx, actor, r) {
		return models.Recording{}, plugin.ErrForbidden
	}
	if r.Status == models.RecordingActive {
		return models.Recording{}, plugin.ErrConflict
	}
	if r.StorageKey != "" {
		if err := s.blobs.Delete(ctx, r.StorageKey); err != nil {
			return models.Recording{}, err
		}
	}
	if err := s.recs.Delete(ctx, id); err != nil {
		return models.Recording{}, err
	}
	return r, nil
}

// Cleanup deletes the blobs of recordings expired as of now and marks their
// metadata discarded. It is a no-op for already-discarded rows.
func (s *RecordingService) Cleanup(ctx context.Context, now time.Time) (int, error) {
	expired, err := s.recs.List(ctx, store.RecordingFilter{ExpiredBefore: now})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range expired {
		if r.Status == models.RecordingDiscarded || r.Status == models.RecordingActive {
			continue
		}
		if r.StorageKey != "" {
			if err := s.blobs.Delete(ctx, r.StorageKey); err != nil {
				return n, err
			}
		}
		r.Status = models.RecordingDiscarded
		r.Error = "expired"
		if err := s.recs.Update(ctx, &r); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
