package sftp

import "os"

// Client exposes the subset of SFTP operations required by the platform.
// Additional methods can be added as new features are implemented.
type Client interface {
	ReadDir(path string) ([]os.FileInfo, error)
	Stat(path string) (os.FileInfo, error)
}

// Provider yields SFTP clients and release callbacks tied to an active session.
type Provider interface {
	AcquireSFTP() (Client, func() error, error)
}
