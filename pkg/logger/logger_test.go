package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestInitSetsLevel(t *testing.T) {
	require.NoError(t, Init("debug"))

	if !Logger().Core().Enabled(zap.DebugLevel) {
		t.Fatalf("expected debug level to be enabled")
	}
}

func TestWithModuleAddsField(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)

	mu.Lock()
	globalLogger = zap.New(core)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		globalLogger = zap.NewNop()
		mu.Unlock()
	})

	WithModule("auth").Info("test message")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	found := false
	for _, field := range entry.Context {
		if field.Key == "module" && field.String == "auth" {
			found = true
		}
	}

	if !found {
		t.Fatalf("expected module field to be included in log context")
	}
}

func TestSyncDoesNotError(t *testing.T) {
	require.NoError(t, Init("info"))
	_ = Sync()
}
