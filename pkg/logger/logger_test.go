package logger

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestInitConfiguresGlobalLogger(t *testing.T) {
	t.Cleanup(func() {
		globalLogger = zap.NewNop()
	})

	if err := Init("debug"); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("expected Logger to return non-nil logger")
	}
	if !logger.Core().Enabled(zap.DebugLevel) {
		t.Fatal("expected logger to enable debug level")
	}
}

func TestLoggingHelpersEmitEntries(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	t.Cleanup(func() {
		globalLogger = zap.NewNop()
	})
	globalLogger = zap.New(core)

	Info("info message", zap.String("k", "v"))
	Error("error message")
	Warn("warn message")
	Debug("debug message")

	if recorded.Len() != 4 {
		t.Fatalf("expected 4 log entries, got %d", recorded.Len())
	}

	messages := recorded.All()
	want := []string{"info message", "error message", "warn message", "debug message"}
	for i, entry := range messages {
		if entry.Message != want[i] {
			t.Fatalf("entry %d message = %q, want %q", i, entry.Message, want[i])
		}
	}
	if field := messages[0].ContextMap()["k"]; field != "v" {
		t.Fatalf("expected field \"k\" to equal \"v\", got %v", field)
	}
}

func TestWithModuleAttachesModuleField(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	t.Cleanup(func() {
		globalLogger = zap.NewNop()
	})
	globalLogger = zap.New(core)

	logger := WithModule("api")
	logger.Info("module test")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if module := entries[0].ContextMap()["module"]; module != "api" {
		t.Fatalf("expected module field to be \"api\", got %v", module)
	}
}
