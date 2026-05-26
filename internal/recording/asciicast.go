package recording

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// asciicastEnvAllowlist is the only environment a recording header may carry —
// terminal-shaping variables, never anything that could leak secrets.
var asciicastEnvAllowlist = []string{"TERM", "SHELL", "LANG"}

const asciicastIdleTimeLimit = 2.0

// asciicastRecorder encodes terminal events as asciicast v2: a JSON header line
// followed by newline-delimited `[time, code, data]` event arrays. Writes are
// incremental and flushed per line, so a recording stays valid if the session
// ends abruptly. Output is `o`, input `i`, resize `r` ("{cols}x{rows}").
type asciicastRecorder struct {
	mu  sync.Mutex
	w   io.Writer
	err error
}

// NewAsciicastRecorder writes the header and returns a terminal Recorder.
func NewAsciicastRecorder(w io.Writer, info StartInfo) (Recorder, error) {
	cols := info.Cols
	if cols <= 0 {
		cols = 80
	}
	rows := info.Rows
	if rows <= 0 {
		rows = 24
	}
	header := map[string]any{
		"version":         2,
		"width":           cols,
		"height":          rows,
		"idle_time_limit": asciicastIdleTimeLimit,
	}
	if !info.Start.IsZero() {
		header["timestamp"] = info.Start.Unix()
	}
	if info.Title != "" {
		header["title"] = info.Title
	}
	if env := allowedEnv(info.Env); len(env) > 0 {
		header["env"] = env
	}

	r := &asciicastRecorder{w: w}
	if err := r.writeJSON(header); err != nil {
		return nil, err
	}
	return r, nil
}

func allowedEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, k := range asciicastEnvAllowlist {
		if v, ok := env[k]; ok {
			out[k] = v
		}
	}
	return out
}

func (r *asciicastRecorder) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return r.writeLine(b)
}

func (r *asciicastRecorder) writeLine(b []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if _, err := r.w.Write(append(b, '\n')); err != nil {
		r.err = err
		return err
	}
	return nil
}

func (r *asciicastRecorder) event(ts time.Duration, code, data string) error {
	return r.writeJSON([]any{ts.Seconds(), code, data})
}

func (r *asciicastRecorder) WriteOutput(ts time.Duration, p []byte) error {
	return r.event(ts, "o", string(p))
}

func (r *asciicastRecorder) WriteInput(ts time.Duration, p []byte) error {
	return r.event(ts, "i", string(p))
}

func (r *asciicastRecorder) Resize(ts time.Duration, cols, rows int) error {
	return r.event(ts, "r", fmt.Sprintf("%dx%d", cols, rows))
}

func (r *asciicastRecorder) Close() error { return nil }
