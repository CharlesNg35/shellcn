package ai_test

import (
	"context"
	"sync"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai"
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

// scriptedProvider records the tools it was handed, calls the first one, then
// answers — simulating a model that lists resources via a tool.
type scriptedProvider struct {
	gotTools []engine.ToolSpec
}

func (p *scriptedProvider) Models(context.Context) ([]engine.ModelInfo, error) { return nil, nil }

func (p *scriptedProvider) Stream(ctx context.Context, req engine.ChatRequest, exec engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	p.gotTools = req.Tools
	out := make(chan engine.StreamEvent, 8)
	go func() {
		defer close(out)
		if len(req.Tools) > 0 && exec != nil {
			call := engine.ToolCall{ID: "1", Name: req.Tools[0].Name, Input: map[string]any{}}
			out <- engine.StreamEvent{Type: engine.EventToolCall, ToolID: call.ID, ToolName: call.Name}
			res, err := exec.Execute(ctx, call)
			out <- engine.StreamEvent{Type: engine.EventToolResult, ToolID: call.ID, ToolName: call.Name, Output: res, Err: errStr(err)}
		}
		out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: "done"}
		out <- engine.StreamEvent{Type: engine.EventDone}
	}()
	return out, nil
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// demoPlugin mirrors the tools-package fake but lives here to keep the test
// self-contained.
type demoPlugin struct{}

func (demoPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "demo", Version: "0", Title: "Demo",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
}

func (demoPlugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "demo.list", Method: plugin.MethodGet, Risk: plugin.RiskSafe, Permission: "demo.read", AuditEvent: "demo.list",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.delete", Method: plugin.MethodDelete, Risk: plugin.RiskDestructive, Permission: "demo.delete", AuditEvent: "demo.delete",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
	}
}

func (demoPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, plugin.ErrNotSupported
}

type recordingInvoker struct {
	mu     sync.Mutex
	calls  []string
	asUser string
}

func (r *recordingInvoker) InvokeRoute(_ context.Context, user models.User, _, routeID string, _ map[string]string, _ []byte) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, routeID)
	r.asUser = user.ID
	return map[string]any{"items": []string{"a", "b"}}, nil
}

func TestAgentListsResourcesViaTools(t *testing.T) {
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	st := store.NewMemory()
	global := config.AIConfig{Kind: "openai", Name: "Shared", APIKey: "k", Model: "gpt-4o"}
	providers := aiconfig.New(st.AIProviders, vault, global)

	reg := pluginregistry.New()
	reg.MustRegister(demoPlugin{})
	inv := &recordingInvoker{}

	prov := &scriptedProvider{}
	svc := ai.New(providers, global, reg, inv, nil, modelreg.New(modelreg.WithURLs("", ""))).WithProviderFactory(
		func(context.Context, models.AIProviderKind, string, string, string) (engine.Provider, error) {
			return prov, nil
		},
	)

	var events []engine.StreamEvent
	err := svc.Run(context.Background(), ai.RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		ConnectionTitle: "prod", AIMode: "read_only", UserMessage: "list resources",
	}, func(ev engine.StreamEvent) { events = append(events, ev) })
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Read-only: the model was offered the safe tool + the investigate subagent,
	// never the destructive one.
	offered := map[string]bool{}
	for _, tspec := range prov.gotTools {
		offered[tspec.Name] = true
	}
	if !offered["demo_list"] || !offered["investigate"] {
		t.Fatalf("read-only should expose demo_list + investigate, got %+v", prov.gotTools)
	}
	if offered["demo_delete"] {
		t.Fatalf("read-only must not expose the destructive tool: %+v", prov.gotTools)
	}

	// The tool call ran through the invoker as the signed-in user.
	if len(inv.calls) != 1 || inv.calls[0] != "demo.list" {
		t.Fatalf("expected demo.list invoked, got %v", inv.calls)
	}
	if inv.asUser != "u1" {
		t.Fatalf("tool must run as the user, got %q", inv.asUser)
	}

	// The stream surfaced tool + text + done events.
	var sawTool, sawDone bool
	for _, e := range events {
		switch e.Type {
		case engine.EventToolResult:
			sawTool = true
		case engine.EventDone:
			sawDone = true
		}
	}
	if !sawTool || !sawDone {
		t.Fatalf("missing tool/done events: %+v", events)
	}
}

func TestTurnPersistsConversationHistory(t *testing.T) {
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	st := store.NewMemory()
	global := config.AIConfig{Kind: "openai", Name: "Shared", APIKey: "k", Model: "gpt-4o"}
	providers := aiconfig.New(st.AIProviders, vault, global)
	mem := memory.New(st.AIConversations, st.AIMessages)

	reg := pluginregistry.New()
	reg.MustRegister(demoPlugin{})

	svc := ai.New(providers, global, reg, &recordingInvoker{}, mem, modelreg.New(modelreg.WithURLs("", ""))).WithProviderFactory(
		func(context.Context, models.AIProviderKind, string, string, string) (engine.Provider, error) {
			return &scriptedProvider{}, nil
		},
	)

	conv, err := mem.Create(context.Background(), "u1", "c1", "", "gpt-4o")
	if err != nil {
		t.Fatalf("create conv: %v", err)
	}

	err = svc.Run(context.Background(), ai.RunInput{
		User: models.User{ID: "u1"}, ConnID: "c1", Protocol: "demo",
		AIMode: "read_only", ConversationID: conv.ID, UserMessage: "list resources",
	}, func(engine.StreamEvent) {})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	msgs, err := mem.Messages(context.Background(), "u1", conv.ID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want user + assistant persisted, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "list resources" {
		t.Fatalf("user message not persisted: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 {
		t.Fatalf("assistant message/tool calls not persisted: %+v", msgs[1])
	}

	// Auto-title fired on the first exchange.
	got, _ := mem.Get(context.Background(), "u1", conv.ID)
	if !got.AutoTitled {
		t.Fatal("conversation should be auto-titled after first message")
	}
}
