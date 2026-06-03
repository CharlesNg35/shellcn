package service_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func newRecordingSvc(t *testing.T) (*service.RecordingService, *store.Store, recording.BlobStore) {
	t.Helper()
	st := store.NewMemory()
	bs, err := recording.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	return service.NewRecordingService(st.Recordings, bs), st, bs
}

var (
	admin    = models.User{ID: "admin", Roles: []models.Role{models.RoleAdmin}}
	op       = models.User{ID: "op", Roles: []models.Role{models.RoleOperator}}
	stranger = models.User{ID: "stranger", Roles: []models.Role{models.RoleViewer}}
)

func seedRecording(t *testing.T, st *store.Store, bs recording.BlobStore, id, userID, connID string, status models.RecordingStatus) {
	t.Helper()
	ctx := context.Background()
	key := recording.StorageKey(connID, id, plugin.FormatAsciicastV2)
	if status == models.RecordingFinalized {
		w, _ := bs.Create(ctx, key)
		_, _ = w.Write([]byte("[2,80,24]\n"))
		_ = w.Close()
	}
	r := &models.Recording{
		ID: id, UserID: userID, ConnectionID: connID, Protocol: "ssh", Class: "terminal",
		Format: "asciicast_v2", Status: status, StartedAt: time.Now(), StorageKey: key,
	}
	if err := st.Recordings.Create(ctx, r); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestRecordingAuthZScope(t *testing.T) {
	svc, st, bs := newRecordingSvc(t)
	ctx := context.Background()

	seedRecording(t, st, bs, "r-op", "op", "c-op", models.RecordingFinalized)
	seedRecording(t, st, bs, "r-other", "other", "c-other", models.RecordingFinalized)
	seedRecording(t, st, bs, "r-managed", "other", "c-managed", models.RecordingFinalized)
	seedRecording(t, st, bs, "r-owned-conn", "other", "c-owned", models.RecordingFinalized)
	_ = st.Connections.Create(ctx, &models.Connection{ID: "c-owned", OwnerID: "op"})
	_ = st.Grants.Create(ctx, &models.Grant{ID: "g1", ConnectionID: "c-managed", SubjectID: "op", Access: models.AccessManage})

	// Admin has no special access: recordings are private to their creator, so an
	// admin sees only their own (none) and a user filter is forced to self.
	if all, _ := svc.List(ctx, admin, store.RecordingFilter{}); len(all) != 0 {
		t.Fatalf("admin list: want 0 (own only), got %d", len(all))
	}
	if byUser, _ := svc.List(ctx, admin, store.RecordingFilter{UserID: "other"}); len(byUser) != 0 {
		t.Fatalf("admin must not drill into another user: want 0, got %d", len(byUser))
	}

	// op sees only their own recordings. Connection ownership and manage grants do
	// not expose other users' recordings.
	got, _ := svc.List(ctx, op, store.RecordingFilter{})
	ids := map[string]bool{}
	for _, r := range got {
		ids[r.ID] = true
	}
	if len(got) != 1 || !ids["r-op"] {
		t.Fatalf("op scoped list: want {r-op}, got %+v", ids)
	}

	// stranger sees nothing.
	if got, _ := svc.List(ctx, stranger, store.RecordingFilter{}); len(got) != 0 {
		t.Fatalf("stranger list: want 0, got %d", len(got))
	}

	// Get authz.
	if _, err := svc.Get(ctx, op, "r-op"); err != nil {
		t.Errorf("op get own: %v", err)
	}
	if _, err := svc.Get(ctx, op, "r-managed"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("op get managed other recording: want forbidden, got %v", err)
	}
	if _, err := svc.Get(ctx, op, "r-owned-conn"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("op get owned connection other recording: want forbidden, got %v", err)
	}
	if _, err := svc.Get(ctx, op, "r-other"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("op get unrelated: want forbidden, got %v", err)
	}
	if _, err := svc.Get(ctx, stranger, "r-other"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("stranger get: want forbidden, got %v", err)
	}
	if _, err := svc.Get(ctx, admin, "r-other"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("admin get another's recording: want forbidden, got %v", err)
	}
}

func TestRecordingContentAndDelete(t *testing.T) {
	svc, st, bs := newRecordingSvc(t)
	ctx := context.Background()

	seedRecording(t, st, bs, "r-fin", "op", "c-op", models.RecordingFinalized)
	seedRecording(t, st, bs, "r-active", "op", "c-op", models.RecordingActive)

	// Content of a finalized recording streams the blob.
	rc, rec, err := svc.Content(ctx, op, "r-fin")
	if err != nil {
		t.Fatalf("content: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if len(data) == 0 || rec.Format != "asciicast_v2" {
		t.Fatalf("unexpected content: %q rec=%+v", data, rec)
	}

	// Content of a still-active recording is unavailable.
	if _, _, err := svc.Content(ctx, op, "r-active"); !errors.Is(err, plugin.ErrUnavailable) {
		t.Errorf("active content: want unavailable, got %v", err)
	}
	if _, err := svc.Delete(ctx, op, "r-active"); !errors.Is(err, plugin.ErrConflict) {
		t.Errorf("active delete: want conflict, got %v", err)
	}

	// A stranger cannot delete.
	if _, err := svc.Delete(ctx, stranger, "r-fin"); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("stranger delete: want forbidden, got %v", err)
	}

	// The owner can; blob and metadata both go away.
	if _, err := svc.Delete(ctx, op, "r-fin"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := st.Recordings.Get(ctx, "r-fin"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("metadata not deleted: %v", err)
	}
	if _, err := bs.Open(ctx, recording.StorageKey("c-op", "r-fin", plugin.FormatAsciicastV2)); err == nil {
		t.Error("blob not deleted")
	}
}

func TestRecordingRetentionCleanup(t *testing.T) {
	svc, st, bs := newRecordingSvc(t)
	ctx := context.Background()
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	mk := func(id string, expires *time.Time, status models.RecordingStatus) {
		key := recording.StorageKey("c-op", id, plugin.FormatAsciicastV2)
		w, _ := bs.Create(ctx, key)
		_, _ = w.Write([]byte("data"))
		_ = w.Close()
		_ = st.Recordings.Create(ctx, &models.Recording{
			ID: id, UserID: "op", ConnectionID: "c-op", Protocol: "ssh", Class: "terminal",
			Format: "asciicast_v2", Status: status, StartedAt: now,
			StorageKey: key, ExpiresAt: expires,
		})
	}
	mk("r-expired", &past, models.RecordingFinalized)
	mk("r-future", &future, models.RecordingFinalized)
	mk("r-active", &past, models.RecordingActive)
	mk("r-kept", nil, models.RecordingFinalized) // no expiry → retained

	n, err := svc.Cleanup(ctx, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if n != 1 {
		t.Fatalf("cleanup count: want 1, got %d", n)
	}

	expired, _ := st.Recordings.Get(ctx, "r-expired")
	if expired.Status != models.RecordingDiscarded {
		t.Errorf("expired not marked discarded: %s", expired.Status)
	}
	if _, err := bs.Open(ctx, recording.StorageKey("c-op", "r-expired", plugin.FormatAsciicastV2)); err == nil {
		t.Error("expired blob not deleted")
	}
	// Untouched.
	if kept, _ := st.Recordings.Get(ctx, "r-kept"); kept.Status != models.RecordingFinalized {
		t.Errorf("kept recording changed: %s", kept.Status)
	}
	if active, _ := st.Recordings.Get(ctx, "r-active"); active.Status != models.RecordingActive {
		t.Errorf("active recording should not be cleaned: %s", active.Status)
	}
	if _, err := bs.Open(ctx, recording.StorageKey("c-op", "r-future", plugin.FormatAsciicastV2)); err != nil {
		t.Errorf("future blob wrongly deleted: %v", err)
	}

	// Re-running cleanup does nothing (already discarded).
	if n, _ := svc.Cleanup(ctx, now); n != 0 {
		t.Errorf("second cleanup: want 0, got %d", n)
	}
}
