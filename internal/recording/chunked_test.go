package recording

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func desktopInfo() StreamInfo {
	m := plugin.Manifest{
		Name:    "vnc",
		Streams: []plugin.Stream{{ID: "vnc.screen", Kind: plugin.StreamDesktop, RouteID: "vnc.screen"}},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas},
			StreamIDs: []string{"vnc.screen"},
		}},
	}
	return StreamInfo{
		User: models.User{ID: "u1", Username: "u"},
		Connection: models.Connection{
			ID: "c1", Protocol: "vnc",
			Recording: map[string]string{"desktop": string(plugin.PolicyManual)},
		},
		Manifest: m, Route: plugin.Route{ID: "vnc.screen"}, StreamID: "vnc.screen",
	}
}

func newChunkEngine(t *testing.T) (*Engine, *store.Store, BlobStore) {
	t.Helper()
	st := store.NewMemory()
	blobs, err := NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("blobs: %v", err)
	}
	return NewEngine(Options{Store: st.Recordings, Blobs: blobs}), st, blobs
}

func TestChunkedLifecycle(t *testing.T) {
	e, st, blobs := newChunkEngine(t)
	ctx := context.Background()

	row, err := e.BeginChunked(ctx, desktopInfo(), plugin.FormatWebMCanvas)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if row.Class != "desktop" || row.Format != "webm_canvas" || row.Authoritative {
		t.Fatalf("browser capture must be non-authoritative desktop: %+v", row)
	}

	if err := e.AppendChunk(ctx, row.ID, "u1", 0, []byte("AAAA")); err != nil {
		t.Fatalf("chunk 0: %v", err)
	}
	// Out-of-order chunk rejected.
	if err := e.AppendChunk(ctx, row.ID, "u1", 2, []byte("X")); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("out-of-order chunk: want invalid input, got %v", err)
	}
	if err := e.AppendChunk(ctx, row.ID, "u1", 1, []byte("BBBB")); err != nil {
		t.Fatalf("chunk 1: %v", err)
	}
	// Another user cannot append.
	if err := e.AppendChunk(ctx, row.ID, "intruder", 2, []byte("Z")); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("foreign append: want forbidden, got %v", err)
	}

	fin, err := e.FinalizeChunked(ctx, row.ID, "u1")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if fin.Status != models.RecordingFinalized || fin.Size != 8 || fin.Checksum == "" {
		t.Fatalf("finalized metadata wrong: %+v", fin)
	}

	rc, _ := blobs.Open(ctx, fin.StorageKey)
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "AAAABBBB" {
		t.Fatalf("blob content: %q", data)
	}

	// Finalized recording persists.
	stored, _ := st.Recordings.Get(ctx, row.ID)
	if stored.Status != models.RecordingFinalized {
		t.Errorf("store status: %s", stored.Status)
	}
}

func TestChunkedAbortDiscardsBlob(t *testing.T) {
	e, st, blobs := newChunkEngine(t)
	ctx := context.Background()
	row, _ := e.BeginChunked(ctx, desktopInfo(), plugin.FormatWebMCanvas)
	_ = e.AppendChunk(ctx, row.ID, "u1", 0, []byte("partial"))

	if err := e.AbortChunked(ctx, row.ID, "u1"); err != nil {
		t.Fatalf("abort: %v", err)
	}
	if _, err := blobs.Open(ctx, row.StorageKey); err == nil {
		t.Error("aborted blob should be deleted")
	}
	stored, _ := st.Recordings.Get(ctx, row.ID)
	if stored.Status != models.RecordingDiscarded {
		t.Errorf("aborted recording status: %s", stored.Status)
	}
}

func TestChunkedFinalizeRejectsEmptyRecording(t *testing.T) {
	e, _, _ := newChunkEngine(t)
	ctx := context.Background()
	row, err := e.BeginChunked(ctx, desktopInfo(), plugin.FormatWebMCanvas)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := e.FinalizeChunked(ctx, row.ID, "u1"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("empty finalize: want invalid input, got %v", err)
	}
}

func TestChunkedRejectsNonDesktopStream(t *testing.T) {
	e, _, _ := newChunkEngine(t)
	if _, err := e.BeginChunked(context.Background(), streamInfo("manual"), plugin.FormatWebMCanvas); !errors.Is(err, plugin.ErrNotSupported) {
		t.Errorf("terminal stream as desktop: want not supported, got %v", err)
	}
}

func TestChunkedRequiresEnabledDesktopPolicy(t *testing.T) {
	e, _, _ := newChunkEngine(t)
	info := desktopInfo()
	info.Connection.Recording = nil
	if _, err := e.BeginChunked(context.Background(), info, plugin.FormatWebMCanvas); !errors.Is(err, plugin.ErrForbidden) {
		t.Errorf("disabled desktop policy: want forbidden, got %v", err)
	}
}

func TestChunkedRejectsUnsupportedFormatUpload(t *testing.T) {
	e, _, _ := newChunkEngine(t)
	info := desktopInfo()
	format := plugin.RecordingFormat("unsupported_native")
	info.Manifest.Recording[0].Formats = []plugin.RecordingFormat{plugin.FormatWebMCanvas, format}
	if _, err := e.BeginChunked(context.Background(), info, format); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("unsupported chunk upload: want invalid input, got %v", err)
	}
}
