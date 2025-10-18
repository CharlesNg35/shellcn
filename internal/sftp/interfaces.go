package sftp

import (
	"io"
	"os"
)

// ReadableFile provides seekable read access to remote files.
type ReadableFile interface {
	io.ReadCloser
	io.Seeker
}

// WritableFile exposes write and seek operations for remote files.
type WritableFile interface {
	io.WriteCloser
	io.Writer
	io.WriterAt
	io.Seeker
}

// Client exposes the subset of SFTP operations required by the platform.
type Client interface {
	ReadDir(path string) ([]os.FileInfo, error)
	Stat(path string) (os.FileInfo, error)
	Open(path string) (ReadableFile, error)
	OpenFile(path string, flag int) (WritableFile, error)
	Create(path string) (WritableFile, error)
	MkdirAll(path string) error
	Remove(path string) error
	RemoveDirectory(path string) error
	Rename(oldPath, newPath string) error
	Truncate(path string, size int64) error
	RealPath(path string) (string, error)
}

// Provider yields SFTP clients and release callbacks tied to an active session.
type Provider interface {
	AcquireSFTP() (Client, func() error, error)
}
