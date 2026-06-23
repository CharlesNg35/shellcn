package kubernetes

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"k8s.io/klog/v2"
)

var routeKlogOnce sync.Once

// routeKlog sends client-go's klog through the process's structured logger and
// drops its non-actionable keepalive churn — it logs "Websocket Ping failed"
// every few seconds and continues on a half-open exec/agent-tunnel stream. klog
// is a client-go (kubernetes) concern, so the wiring lives in this plugin rather
// than the plugin-agnostic core.
func routeKlog() {
	routeKlogOnce.Do(func() {
		klog.SetSlogLogger(slog.New(&dropHandler{
			Handler: slog.Default().Handler(),
			drop:    []string{"Websocket Ping failed"},
		}))
	})
}

// dropHandler discards records whose message contains a configured substring and
// passes everything else through unchanged.
type dropHandler struct {
	slog.Handler
	drop []string
}

func (h *dropHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, s := range h.drop {
		if strings.Contains(r.Message, s) {
			return nil
		}
	}
	return h.Handler.Handle(ctx, r)
}

func (h *dropHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &dropHandler{Handler: h.Handler.WithAttrs(attrs), drop: h.drop}
}

func (h *dropHandler) WithGroup(name string) slog.Handler {
	return &dropHandler{Handler: h.Handler.WithGroup(name), drop: h.drop}
}
