package ai_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai"
	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
)

type nopInvoker struct{}

func (nopInvoker) InvokeRoute(context.Context, models.User, string, string, map[string]string, []byte) (any, error) {
	return nil, nil
}

func newService(t *testing.T, global config.AIConfig) *ai.Service {
	t.Helper()
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	st := store.NewMemory()
	providers := aiconfig.New(st.AIProviders, vault, global)
	return ai.New(providers, global, plugin.NewRegistry(), nopInvoker{}, nil)
}

func TestAllowedRisks(t *testing.T) {
	ro := ai.AllowedRisks("read_only", true)
	if !ro[plugin.RiskSafe] || ro[plugin.RiskWrite] || ro[plugin.RiskDestructive] {
		t.Fatalf("read_only should expose only safe: %v", ro)
	}
	rw := ai.AllowedRisks("read_write", false)
	if !rw[plugin.RiskSafe] || !rw[plugin.RiskWrite] || rw[plugin.RiskDestructive] {
		t.Fatalf("read_write w/o destructive: %v", rw)
	}
	rwd := ai.AllowedRisks("read_write", true)
	if !rwd[plugin.RiskDestructive] {
		t.Fatal("read_write + allowDestructive should expose destructive")
	}
	// Privileged is never exposed.
	for _, m := range []string{"read_only", "read_write"} {
		if ai.AllowedRisks(m, true)[plugin.RiskPrivileged] {
			t.Fatalf("privileged must never be allowed (%s)", m)
		}
	}
}

func TestRunWithoutProviderErrors(t *testing.T) {
	svc := newService(t, config.AIConfig{})
	if svc.Configured(context.Background(), "u1") {
		t.Fatal("no provider should report not configured")
	}
	err := svc.Run(context.Background(), ai.RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: "read_only", UserMessage: "hi",
	}, func(engine.StreamEvent) {})
	if !errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestConfiguredViaGlobal(t *testing.T) {
	svc := newService(t, config.AIConfig{Kind: "openai", Name: "Shared", APIKey: "k", DefaultModel: "gpt-4o"})
	if !svc.Configured(context.Background(), "u1") {
		t.Fatal("global config should report configured")
	}
}
