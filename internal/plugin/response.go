package plugin

import (
	"io"
	"time"
)

// Download is a route result that the core streams as an attachment instead of
// JSON encoding. The route wrapper still owns authz, audit, and error handling.
type Download struct {
	Name    string
	MIME    string
	Size    int64
	ModTime time.Time
	Body    io.ReadCloser
}
