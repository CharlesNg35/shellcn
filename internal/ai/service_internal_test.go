package ai

import (
	"context"
	"strings"
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
	mu         sync.Mutex
	titleCall  int
	titleCh    chan engine.StreamEvent
	titleText  string
	gotContext string
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

func (p *blockingTitleProvider) Stream(_ context.Context, req engine.ChatRequest, _ engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	out := make(chan engine.StreamEvent, 4)
	if req.MaxSteps != 1 {
		go func() {
			defer close(out)
			out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: "done"}
			out <- engine.StreamEvent{Type: engine.EventDone}
		}()
		return out, nil
	}

	p.mu.Lock()
	p.titleCall++
	if len(req.Messages) > 0 {
		p.gotContext = req.Messages[0].Content
	}
	title := p.titleText
	if p.titleCall > 1 {
		title = "Database Backup Failure"
	}
	p.titleCh = out
	p.mu.Unlock()
	if title != "" {
		go func() {
			defer close(out)
			out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: title}
			out <- engine.StreamEvent{Type: engine.EventDone}
		}()
	}
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

func TestAutoTitleUsesConversationContextAndRetriesUntilResolved(t *testing.T) {
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

	err = svc.Run(context.Background(), RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: models.AIModeReadOnly, ConversationID: conv.ID,
		UserMessage: "why did the database backup fail",
	}, func(engine.StreamEvent) {})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := mem.Get(context.Background(), "u1", conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got.TitleResolved {
		t.Fatalf("first exchange should not resolve title: %+v", got)
	}

	start := time.Now()
	err = svc.Run(context.Background(), RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: models.AIModeReadOnly, ConversationID: conv.ID,
		UserMessage: "what should I clean up",
	}, func(engine.StreamEvent) {})
	provider.closeTitleStream()
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("title generation timeout should not hold the turn open")
	}
	got, err = mem.Get(context.Background(), "u1", conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got.TitleResolved {
		t.Fatalf("failed title generation should leave title unresolved for retry: %+v", got)
	}

	err = svc.Run(context.Background(), RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: models.AIModeReadOnly, ConversationID: conv.ID,
		UserMessage: "retry title",
	}, func(engine.StreamEvent) {})
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	got, err = mem.Get(context.Background(), "u1", conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got.Title != "Database Backup Failure" || !got.TitleResolved {
		t.Fatalf("later title retry should resolve conversation: %+v", got)
	}
	if provider.gotContext == "" || !strings.Contains(provider.gotContext, "user: why did the database backup fail") || !strings.Contains(provider.gotContext, "assistant: done") {
		t.Fatalf("title request should use conversation context, got %q", provider.gotContext)
	}
}
