package recording

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
)

// StartInfo is the metadata a Recorder needs to emit its header.
type StartInfo struct {
	Title  string
	Cols   int
	Rows   int
	Env    map[string]string
	Start  time.Time
	Format plugin.RecordingFormat
}

// Recorder encodes timestamped stream events into its backing writer. Timestamps
// are relative to the recording start (monotonic, non-decreasing). A Recorder is
// driven by a single goroutine (the tap drain loop), so it need not be reentrant.
type Recorder interface {
	WriteOutput(ts time.Duration, p []byte) error
	WriteInput(ts time.Duration, p []byte) error
	Resize(ts time.Duration, cols, rows int) error
	Close() error
}

// RecorderFactory builds a Recorder for one recording, writing to w.
type RecorderFactory func(w io.Writer, info StartInfo) (Recorder, error)

// countingWriter tracks the byte count and running checksum of what it forwards.
type countingWriter struct {
	w io.Writer
	h hash.Hash
	n int64
}

func newCountingWriter(w io.Writer) *countingWriter {
	return &countingWriter{w: w, h: sha256.New()}
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		c.n += int64(n)
		_, _ = c.h.Write(p[:n])
	}
	return n, err
}

func (c *countingWriter) checksum() string { return hex.EncodeToString(c.h.Sum(nil)) }
