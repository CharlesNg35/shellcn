package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

const (
	// RequestIDHeader carries a correlation id across the request boundary.
	RequestIDHeader = "X-Request-Id"

	LogFormatConsole = "console"
	LogFormatJSON    = "json"
	LogFormatText    = "text"
)

type ctxKey struct{}

// LogConfig configures the structured logger.
type LogConfig struct {
	Level  slog.Level
	Format string    // "console", "json", or "text"
	Output io.Writer // defaults to os.Stdout
}

// NewLogger builds the gateway's structured logger. Records logged with a request
// context automatically carry that request's correlation id.
func NewLogger(cfg LogConfig) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	var h slog.Handler
	switch cfg.Format {
	case LogFormatJSON:
		opts := &slog.HandlerOptions{Level: cfg.Level}
		h = slog.NewJSONHandler(out, opts)
	case LogFormatConsole:
		h = tint.NewHandler(out, &tint.Options{
			Level:       cfg.Level,
			TimeFormat:  time.TimeOnly,
			NoColor:     !shouldColor(out),
			ReplaceAttr: consoleAttr,
		})
	default:
		opts := &slog.HandlerOptions{Level: cfg.Level}
		h = slog.NewTextHandler(out, opts)
	}
	return slog.New(&contextHandler{Handler: h})
}

func consoleAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindAny {
		if _, ok := a.Value.Any().(error); ok {
			return tint.Attr(9, a)
		}
	}
	return a
}

func shouldColor(out io.Writer) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	if force := os.Getenv("FORCE_COLOR"); force != "" && force != "0" {
		return true
	}
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// contextHandler adds the request correlation id to every record under a request.
type contextHandler struct{ slog.Handler }

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestIDMiddleware ensures every request carries a correlation id, echoes it
// on the response, and stores it in the context for structured logging.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestID returns the correlation id stored on the context, if any.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
