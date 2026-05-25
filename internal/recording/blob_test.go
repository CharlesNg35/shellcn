package recording_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/charlesng/shellcn/internal/recording"
)

func TestLocalBlobStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	bs, err := recording.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	w, err := bs.Create(ctx, "conn1/rec1.cast")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := w.Write([]byte("hello ")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Append extends the same blob (chunked upload path).
	if err := bs.Append(ctx, "conn1/rec1.cast", []byte("world")); err != nil {
		t.Fatalf("append: %v", err)
	}

	rc, err := bs.Open(ctx, "conn1/rec1.cast")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "hello world" {
		t.Fatalf("round-trip mismatch: %q", data)
	}

	if size, _ := bs.Size(ctx, "conn1/rec1.cast"); size != 11 {
		t.Fatalf("size: want 11, got %d", size)
	}

	if err := bs.Delete(ctx, "conn1/rec1.cast"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Delete of an absent key is not an error.
	if err := bs.Delete(ctx, "conn1/rec1.cast"); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
	if _, err := bs.Open(ctx, "conn1/rec1.cast"); err == nil {
		t.Fatal("open deleted blob should fail")
	}
}

func TestLocalBlobStoreRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	bs, err := recording.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, key := range []string{"../escape", "../../etc/passwd", ""} {
		if _, err := bs.Create(ctx, key); !errors.Is(err, recording.ErrInvalidKey) {
			t.Errorf("key %q: want ErrInvalidKey, got %v", key, err)
		}
	}
}
