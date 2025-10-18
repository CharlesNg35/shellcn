package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFilesystemRecorderStore_CreateAndOpen(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemRecorderStore(dir)
	require.NoError(t, err)

	start := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	writer, err := store.Create(context.Background(), RecordingResource{
		SessionID:  "sess-123",
		ProtocolID: "ssh",
		StartedAt:  start,
	})
	require.NoError(t, err)
	require.NotNil(t, writer)
	require.NotEmpty(t, writer.Path)
	require.NotNil(t, writer.Writer)

	content := []byte("hello world")
	_, err = writer.Writer.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Writer.Close())

	fullPath := filepath.Join(dir, filepath.FromSlash(writer.Path))
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.Equal(t, content, data)

	info, err := store.Stat(context.Background(), writer.Path)
	require.NoError(t, err)
	require.Equal(t, writer.Path, info.Path)
	require.EqualValues(t, len(content), info.Size)

	reader, err := store.Open(context.Background(), writer.Path)
	require.NoError(t, err)
	defer reader.Close()

	read, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, content, read)
}

func TestFilesystemRecorderStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemRecorderStore(dir)
	require.NoError(t, err)

	writer, err := store.Create(context.Background(), RecordingResource{
		SessionID:  "sess-del",
		ProtocolID: "ssh",
		StartedAt:  time.Now(),
	})
	require.NoError(t, err)
	require.NoError(t, writer.Writer.Close())

	require.NoError(t, store.Delete(context.Background(), writer.Path))

	_, err = store.Stat(context.Background(), writer.Path)
	require.Error(t, err)
}

func TestFilesystemRecorderStore_SanitizesPathFragments(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemRecorderStore(dir)
	require.NoError(t, err)

	start := time.Date(2025, 2, 1, 15, 4, 5, 0, time.UTC)
	writer, err := store.Create(context.Background(), RecordingResource{
		SessionID:  "sess../../../Strange ID",
		ProtocolID: "../SSH??",
		StartedAt:  start,
	})
	require.NoError(t, err)
	require.NotNil(t, writer)
	require.NoError(t, writer.Writer.Close())

	require.NotContains(t, writer.Path, "..")
	require.Contains(t, writer.Path, "ssh")
	require.Contains(t, writer.Path, "sess")

	require.Contains(t, writer.Path, "ssh/2025/02")
}
