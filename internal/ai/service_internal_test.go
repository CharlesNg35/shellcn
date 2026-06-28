package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/ai/memory"
	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type internalNopInvoker struct{}

func (internalNopInvoker) InvokeRoute(context.Context, models.User, string, string, map[string]string, []byte) (any, error) {
	return nil, nil
}

type blockingTitleProvider struct {
	mu      sync.Mutex
	calls   int
	titleCh chan engine.StreamEvent
}

type internalDemoPlugin struct{}

func (internalDemoPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Version:    "0",
		Title:      "Demo",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
		},
	}
}

func (internalDemoPlugin) Routes() []plugin.Route { return nil }

func (internalDemoPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, plugin.ErrNotSupported
}

func (p *blockingTitleProvider) Models(context.Context) ([]engine.ModelInfo, error) { return nil, nil }

func (p *blockingTitleProvider) Stream(context.Context, engine.ChatRequest, engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()

	out := make(chan engine.StreamEvent, 4)
	if call == 1 {
		go func() {
			defer close(out)
			out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: "done"}
			out <- engine.StreamEvent{Type: engine.EventDone}
		}()
		return out, nil
	}

	p.mu.Lock()
	p.titleCh = out
	p.mu.Unlock()
	return out, nil
}

func (p *blockingTitleProvider) closeTitleStream() {
	p.mu.Lock()
	ch := p.titleCh
	p.titleCh = nil
	p.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func TestAutoTitleFallsBackWhenTitleProviderStalls(t *testing.T) {
	oldTimeout := titleGenerationTimeout
	titleGenerationTimeout = 10 * time.Millisecond
	t.Cleanup(func() { titleGenerationTimeout = oldTimeout })

	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	st := store.NewMemory()
	global := config.AIConfig{Kind: "openai", Name: "Shared", APIKey: "k", Model: "gpt-4o"}
	providers := aiconfig.New(st.AIProviders, vault, global)
	mem := memory.New(st.AIConversations, st.AIMessages)
	provider := &blockingTitleProvider{}
	reg := pluginregistry.New()
	reg.MustRegister(internalDemoPlugin{})
	svc := New(providers, global, reg, internalNopInvoker{}, mem, modelreg.New(modelreg.WithoutRegistryFetch())).
		WithProviderFactory(func(context.Context, models.AIProviderKind, string, string, string) (engine.Provider, error) {
			return provider, nil
		})

	conv, err := mem.Create(context.Background(), "u1", "c1", "", "gpt-4o")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	start := time.Now()
	err = svc.Run(context.Background(), RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: models.AIModeReadOnly, ConversationID: conv.ID,
		UserMessage: "why did the database backup fail",
	}, func(engine.StreamEvent) {})
	provider.closeTitleStream()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("title generation timeout should not hold the turn open")
	}

	got, err := mem.Get(context.Background(), "u1", conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got.Title != "why did the database backup fail" || !got.TitleResolved {
		t.Fatalf("conversation should use fallback title after timeout: %+v", got)
	}
}
