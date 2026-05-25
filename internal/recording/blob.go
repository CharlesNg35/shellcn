// Package recording owns session-recording capture and storage: the blob store
// for recording bytes, the format-specific recorders, and the core stream
// wrapper that decides — from plugin capability + connection policy — whether a
// stream is recorded. Recording metadata lives in the control-plane store; the
// bytes live behind the BlobStore interface so the backend is replaceable.
package recording

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrInvalidKey is returned when a storage key would escape the blob root.
var ErrInvalidKey = errors.New("recording: invalid storage key")

// BlobStore stores recording bytes under server-generated keys. The local
// filesystem backend is the default; an object-storage backend implements the
// same interface without touching callers.
type BlobStore interface {
	// Create opens key for writing, truncating any existing object.
	Create(ctx context.Context, key string) (io.WriteCloser, error)
	// Append adds data to key, creating it if absent (chunked desktop uploads).
	Append(ctx context.Context, key string, data []byte) error
	// Open opens key for reading.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Size returns the byte length of key.
	Size(ctx context.Context, key string) (int64, error)
	// Delete removes key; absence is not an error.
	Delete(ctx context.Context, key string) error
}

// LocalBlobStore stores blobs as files under a root directory.
type LocalBlobStore struct {
	root string
}

// NewLocalBlobStore creates the root directory and returns a filesystem store.
func NewLocalBlobStore(root string) (*LocalBlobStore, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &LocalBlobStore{root: abs}, nil
}

// resolve maps a key to an absolute path, rejecting empty keys and any traversal
// (".." segments or absolute paths) before it can escape the blob root.
func (s *LocalBlobStore) resolve(key string) (string, error) {
	if key == "" || strings.Contains(key, "\x00") {
		return "", ErrInvalidKey
	}
	slash := filepath.ToSlash(key)
	if slices.Contains(strings.Split(slash, "/"), "..") {
		return "", ErrInvalidKey
	}
	full := filepath.Join(s.root, filepath.FromSlash(slash))
	if full != s.root && !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return full, nil
}

func (s *LocalBlobStore) Create(_ context.Context, key string) (io.WriteCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
}

func (s *LocalBlobStore) Append(_ context.Context, key string, data []byte) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(data)
	return err
}

func (s *LocalBlobStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *LocalBlobStore) Size(_ context.Context, key string) (int64, error) {
	path, err := s.resolve(key)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *LocalBlobStore) Delete(_ context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
