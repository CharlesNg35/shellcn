package plugin

import (
	"io"
	"time"
)

// Download is a route result the core streams over HTTP instead of JSON encoding.
// A handler sets exactly one byte source: Seeker (full Range via http.ServeContent),
// OpenRange (single-range for offset-but-not-seek backends), or Body (full, no Range).
type Download struct {
	Name    string
	MIME    string
	Size    int64 // -1 when unknown
	ModTime time.Time
	Inline  bool // Content-Disposition: inline vs attachment

	Body      io.ReadCloser
	Seeker    io.ReadSeekCloser
	OpenRange func(offset, length int64) (io.ReadCloser, error)
}
