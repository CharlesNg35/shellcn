package extplugin

import (
	"context"
	"io"
	"log"
	"log/slog"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
)

// slogHCLog adapts the gateway's *slog.Logger to the hclog.Logger interface that
// go-plugin expects. go-plugin reads each plugin subprocess's stderr and routes
// it (parsed for level) through this logger, and the Manager logs its own
// supervisor events through it too, so external-plugin output lands in the
// gateway's structured logs instead of being discarded.
type slogHCLog struct {
	log   *slog.Logger
	name  string
	level hclog.Level
}

// newSlogHCLog wraps a slog logger as an hclog logger tagged as the plugin
// component. A nil logger yields a no-op.
func newSlogHCLog(l *slog.Logger) hclog.Logger {
	if l == nil {
		return hclog.NewNullLogger()
	}
	return &slogHCLog{log: l.With(slog.String("component", "extplugin")), level: hclog.Trace}
}

func toSlogLevel(l hclog.Level) slog.Level {
	switch l {
	case hclog.Error:
		return slog.LevelError
	case hclog.Warn:
		return slog.LevelWarn
	case hclog.Info:
		return slog.LevelInfo
	default: // Trace, Debug, NoLevel
		return slog.LevelDebug
	}
}

func (h *slogHCLog) enabled(l hclog.Level) bool {
	return h.level != hclog.Off && l >= h.level
}

func (h *slogHCLog) Log(level hclog.Level, msg string, args ...any) {
	if !h.enabled(level) {
		return
	}
	h.log.Log(context.Background(), toSlogLevel(level), strings.TrimRight(msg, "\n"), args...)
}

func (h *slogHCLog) Trace(msg string, args ...any) { h.Log(hclog.Trace, msg, args...) }
func (h *slogHCLog) Debug(msg string, args ...any) { h.Log(hclog.Debug, msg, args...) }
func (h *slogHCLog) Info(msg string, args ...any)  { h.Log(hclog.Info, msg, args...) }
func (h *slogHCLog) Warn(msg string, args ...any)  { h.Log(hclog.Warn, msg, args...) }
func (h *slogHCLog) Error(msg string, args ...any) { h.Log(hclog.Error, msg, args...) }

func (h *slogHCLog) IsTrace() bool { return h.enabled(hclog.Trace) }
func (h *slogHCLog) IsDebug() bool { return h.enabled(hclog.Debug) }
func (h *slogHCLog) IsInfo() bool  { return h.enabled(hclog.Info) }
func (h *slogHCLog) IsWarn() bool  { return h.enabled(hclog.Warn) }
func (h *slogHCLog) IsError() bool { return h.enabled(hclog.Error) }

func (h *slogHCLog) ImpliedArgs() []any { return nil }

func (h *slogHCLog) With(args ...any) hclog.Logger {
	return &slogHCLog{log: h.log.With(args...), name: h.name, level: h.level}
}

func (h *slogHCLog) Name() string { return h.name }

func (h *slogHCLog) Named(name string) hclog.Logger {
	full := name
	if h.name != "" {
		full = h.name + "." + name
	}
	return h.named(full)
}

func (h *slogHCLog) ResetNamed(name string) hclog.Logger { return h.named(name) }

func (h *slogHCLog) named(full string) hclog.Logger {
	return &slogHCLog{log: h.log.With(slog.String("plugin", full)), name: full, level: h.level}
}

func (h *slogHCLog) SetLevel(level hclog.Level) { h.level = level }
func (h *slogHCLog) GetLevel() hclog.Level      { return h.level }

func (h *slogHCLog) StandardLogger(*hclog.StandardLoggerOptions) *log.Logger {
	return log.New(h.StandardWriter(nil), "", 0)
}

func (h *slogHCLog) StandardWriter(*hclog.StandardLoggerOptions) io.Writer {
	return stdWriter{h}
}

// stdWriter routes stdlib-logger lines to the adapter at info level.
type stdWriter struct{ h *slogHCLog }

func (w stdWriter) Write(p []byte) (int, error) {
	w.h.Info(string(p))
	return len(p), nil
}
