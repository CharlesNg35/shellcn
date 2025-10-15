package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	_ RecorderStore = (*FilesystemRecorderStore)(nil)
)

// RecorderStore abstracts the underlying storage for session recording artifacts.
type RecorderStore interface {
	// Create allocates a new writable object for the supplied recording resource.
	Create(ctx context.Context, resource RecordingResource) (*RecordingWriter, error)
	// Open returns a readable stream for the stored recording at the given path.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	// Stat returns metadata for the stored object located at path.
	Stat(ctx context.Context, path string) (RecordingFileInfo, error)
	// Delete removes the stored object at path.
	Delete(ctx context.Context, path string) error
}

// RecordingResource identifies the session/protocol tuple associated with a recording.
type RecordingResource struct {
	SessionID  string
	ProtocolID string
	StartedAt  time.Time
}

// RecordingWriter represents a writable handle created by the RecorderStore.
type RecordingWriter struct {
	Path   string
	Writer io.WriteCloser
}

// RecordingFileInfo captures size and timestamp metadata for stored recordings.
type RecordingFileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// FilesystemRecorderStore persists recordings on the local filesystem.
type FilesystemRecorderStore struct {
	root string
}

// NewFilesystemRecorderStore initialises a filesystem-backed recorder store rooted at dir.
func NewFilesystemRecorderStore(dir string) (*FilesystemRecorderStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("recorder store: root directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("recorder store: ensure root directory: %w", err)
	}
	return &FilesystemRecorderStore{root: dir}, nil
}

// Create opens a new recording file for writing, organising directories by protocol/year/month.
func (s *FilesystemRecorderStore) Create(_ context.Context, resource RecordingResource) (*RecordingWriter, error) {
	if s == nil {
		return nil, errors.New("recorder store: store not initialised")
	}
	sessionID := strings.TrimSpace(resource.SessionID)
	if sessionID == "" {
		return nil, errors.New("recorder store: session id is required")
	}
	protocolID := sanitizePathFragment(resource.ProtocolID)
	if protocolID == "" {
		protocolID = "unknown"
	}
	startedAt := resource.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	year := fmt.Sprintf("%04d", startedAt.UTC().Year())
	month := fmt.Sprintf("%02d", int(startedAt.UTC().Month()))

	dir := filepath.Join(s.root, protocolID, year, month)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("recorder store: mkdir %s: %w", dir, err)
	}

	filename := fmt.Sprintf("%s-%s.cast.gz", sanitizePathFragment(sessionID), startedAt.UTC().Format("20060102T150405Z"))
	fullPath := filepath.Join(dir, filename)

	fh, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("recorder store: create file: %w", err)
	}

	return &RecordingWriter{
		Path:   s.relative(fullPath),
		Writer: fh,
	}, nil
}

// Open returns a reader for the stored recording.
func (s *FilesystemRecorderStore) Open(_ context.Context, path string) (io.ReadCloser, error) {
	if s == nil {
		return nil, errors.New("recorder store: store not initialised")
	}
	fullPath := s.absolute(path)
	fh, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("recorder store: open file: %w", err)
	}
	return fh, nil
}

// Stat returns file metadata for the stored recording.
func (s *FilesystemRecorderStore) Stat(_ context.Context, path string) (RecordingFileInfo, error) {
	if s == nil {
		return RecordingFileInfo{}, errors.New("recorder store: store not initialised")
	}
	fullPath := s.absolute(path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return RecordingFileInfo{}, fmt.Errorf("recorder store: stat file: %w", err)
	}
	return RecordingFileInfo{
		Path:    s.relative(fullPath),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// Delete removes the stored recording.
func (s *FilesystemRecorderStore) Delete(_ context.Context, path string) error {
	if s == nil {
		return errors.New("recorder store: store not initialised")
	}
	fullPath := s.absolute(path)
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("recorder store: delete file: %w", err)
	}
	return nil
}

func (s *FilesystemRecorderStore) absolute(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.root, filepath.FromSlash(path))
}

func (s *FilesystemRecorderStore) relative(fullPath string) string {
	rel, err := filepath.Rel(s.root, fullPath)
	if err != nil {
		return fullPath
	}
	return filepath.ToSlash(rel)
}

func sanitizePathFragment(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	fragment = strings.ToLower(fragment)
	fragment = strings.ReplaceAll(fragment, "..", "")
	fragment = strings.ReplaceAll(fragment, string(os.PathSeparator), "-")
	fragment = strings.ReplaceAll(fragment, "/", "-")
	fragment = strings.ReplaceAll(fragment, "\\", "-")
	fragment = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, fragment)
	fragment = strings.Trim(fragment, "-")
	if fragment == "" {
		return "recording"
	}
	return fragment
}
