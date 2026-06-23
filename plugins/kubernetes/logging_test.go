package kubernetes

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDropHandlerDropsNoise(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(&dropHandler{
		Handler: slog.NewTextHandler(&buf, nil),
		drop:    []string{"Websocket Ping failed"},
	})
	log.Error("Websocket Ping failed: write tcp: i/o timeout")
	log.Info("genuine client-go warning")
	out := buf.String()
	if strings.Contains(out, "Websocket Ping failed") {
		t.Fatalf("noisy message not dropped: %q", out)
	}
	if !strings.Contains(out, "genuine client-go warning") {
		t.Fatalf("non-noisy message dropped: %q", out)
	}
}
