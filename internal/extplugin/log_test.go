package extplugin

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
)

// TestSlogHCLogForwardsToGateway verifies that plugin output routed through the
// hclog adapter (as go-plugin does for subprocess stderr) lands in the gateway's
// structured logger, tagged with the component and plugin name.
func TestSlogHCLogForwardsToGateway(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	hl := newSlogHCLog(base)
	// go-plugin names the sublogger after the plugin binary, then logs each
	// stderr line through it.
	hl.Named("acme-db").Error("connection refused", "addr", "10.0.0.5:5432")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("decode log line: %v (%q)", err, buf.String())
	}
	if entry["msg"] != "connection refused" {
		t.Errorf("msg = %v, want %q", entry["msg"], "connection refused")
	}
	if entry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entry["level"])
	}
	if entry["component"] != "extplugin" {
		t.Errorf("component = %v, want extplugin", entry["component"])
	}
	if entry["plugin"] != "acme-db" {
		t.Errorf("plugin = %v, want acme-db", entry["plugin"])
	}
	if entry["addr"] != "10.0.0.5:5432" {
		t.Errorf("addr attr = %v, want 10.0.0.5:5432", entry["addr"])
	}
}

func TestSlogHCLogLevelMapping(t *testing.T) {
	var buf bytes.Buffer
	hl := newSlogHCLog(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	hl.Trace("t")
	hl.Debug("d")
	hl.Info("i")
	hl.Warn("w")
	hl.Error("e")

	out := buf.String()
	for _, want := range []string{"level=DEBUG msg=t", "level=DEBUG msg=d", "level=INFO msg=i", "level=WARN msg=w", "level=ERROR msg=e"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// Off silences everything; GetLevel reflects it (go-plugin checks this).
	hl.SetLevel(hclog.Off)
	if hl.GetLevel() != hclog.Off {
		t.Fatalf("GetLevel = %v, want Off", hl.GetLevel())
	}
	buf.Reset()
	hl.Error("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when Off, got %q", buf.String())
	}
}
