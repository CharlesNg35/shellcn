// Package ai is the core AI agent service: it resolves a provider (the user's or
// the shared global config), builds the connection's risk-gated tool set, runs a
// turn through the engine, and relays the stream to transport. It is wired in
// cmd/server like other core services and references no plugin by name.
package ai

import (
	"context"
	"errors"
	"strings"

	"github.com/charlesng35/shellcn/internal/ai/agent"
	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	einoadapter "github.com/charlesng35/shellcn/internal/ai/engine/eino"
	"github.com/charlesng35/shellcn/internal/ai/memory"
	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

// Per-turn caps (cross-cutting limits). Conservative defaults; tunable later.
const (
	defaultMaxSteps     = 12
	defaultMaxOutTokens = 4096
)

var (
	// ErrNotConfigured means neither a user provider nor the shared config is usable.
	ErrNotConfigured = errors.New("ai: no provider configured")
	// ErrProviderUnsupported means the resolved provider kind has no adapter yet.
	ErrProviderUnsupported = errors.New("ai: provider kind not supported")
)

// ProviderFactory builds an engine.Provider for a resolved config. It is the
// injection seam: production uses the eino adapter; tests supply a scripted one.
type ProviderFactory func(ctx context.Context, kind models.AIProviderKind, key, baseURL, model string) (engine.Provider, error)

// Service is the public AI surface used by transport.
type Service struct {
	providers *aiconfig.Service
	global    config.AIConfig
	routes    tools.RouteSource
	invoker   tools.Invoker
	mem       *memory.Store
	factory   ProviderFactory
}

// New wires the provider-config service, the shared global config, the plugin
// registry (route source), the secure route invoker, and conversation memory.
func New(providers *aiconfig.Service, global config.AIConfig, routes tools.RouteSource, invoker tools.Invoker, mem *memory.Store) *Service {
	return &Service{providers: providers, global: global, routes: routes, invoker: invoker, mem: mem, factory: buildProvider}
}

// Conversations exposes the conversation/message store for transport CRUD.
func (s *Service) Conversations() *memory.Store { return s.mem }

// WithProviderFactory overrides the engine adapter (used in tests).
func (s *Service) WithProviderFactory(f ProviderFactory) *Service {
	s.factory = f
	return s
}

// Scope selects which provider config a turn uses: a specific user provider, or
// the shared global config when ProviderID is empty.
type Scope struct {
	ProviderID string
}

// RunInput is one chat turn's request.
type RunInput struct {
	User             models.User
	ConnID           string
	Protocol         string
	ConnectionTitle  string
	AIMode           string // disabled | read_only | read_write
	AllowDestructive bool
	Scope            Scope
	ConversationID   string // when set (with memory wired), history is persisted
	History          []engine.Message
	UserMessage      string
	// RecentOps are pre-formatted recent audit lines injected into the prompt so
	// the agent can explain a just-failed action.
	RecentOps []string
}

// Run executes one turn and relays every event to sink. The caller cancels ctx
// to stop the turn cleanly.
func (s *Service) Run(ctx context.Context, in RunInput, sink func(engine.StreamEvent)) error {
	provider, model, err := s.resolveProvider(ctx, in.User, in.Scope)
	if err != nil {
		return err
	}

	allowed := AllowedRisks(in.AIMode, in.AllowDestructive)
	toolset, err := tools.Build(s.routes, in.Protocol, allowed, s.invoker, in.User, in.ConnID)
	if err != nil {
		return err
	}

	// Expose an investigation subagent (nested read-only run) whenever the
	// connection has any read tools — the primary context-window optimization.
	var exec engine.ToolExecutor = toolset
	specs := toolset.Specs()
	hasSubagent := false
	if ro, err := tools.Build(s.routes, in.Protocol, map[plugin.RiskLevel]bool{plugin.RiskSafe: true}, s.invoker, in.User, in.ConnID); err == nil && len(ro.Specs()) > 0 {
		comp := agent.NewComposite(toolset, agent.NewSubagent("investigate", provider, model, ro, in.Protocol))
		exec = comp
		specs = comp.Specs()
		hasSubagent = true
	}

	names := make([]string, 0, len(specs))
	for _, sp := range specs {
		names = append(names, sp.Name)
	}
	system := agent.SystemPrompt(agent.PromptInput{
		ConnectionTitle: in.ConnectionTitle,
		Protocol:        in.Protocol,
		AIMode:          in.AIMode,
		Tools:           names,
		RecentOps:       in.RecentOps,
		HasSubagent:     hasSubagent,
	})

	// Assemble history. With memory wired + a conversation id, persist the user
	// message and load the compacted context; otherwise use the caller's history.
	var msgs []engine.Message
	persist := s.mem != nil && in.ConversationID != ""
	if persist {
		conv, err := s.mem.Get(ctx, in.User.ID, in.ConversationID)
		if err != nil {
			return err
		}
		if err := s.mem.AppendUser(ctx, in.ConversationID, in.UserMessage); err != nil {
			return err
		}
		summary, history, err := s.mem.History(ctx, in.ConversationID, memory.ContextWindow(model), conv.Summary)
		if err != nil {
			return err
		}
		msgs = history // already includes the just-appended user message
		if summary != "" {
			system += "\n\nConversation memory:\n" + summary
		}
	} else {
		msgs = append(append([]engine.Message{}, in.History...), engine.Message{Role: engine.RoleUser, Content: in.UserMessage})
	}

	ch, err := provider.Stream(ctx, engine.ChatRequest{
		Model:        model,
		System:       system,
		Messages:     msgs,
		Tools:        specs,
		MaxSteps:     defaultMaxSteps,
		MaxOutTokens: defaultMaxOutTokens,
	}, exec)
	if err != nil {
		return err
	}

	acc := &accumulator{}
	relaySink := sink
	if persist {
		relaySink = func(ev engine.StreamEvent) {
			acc.add(ev)
			sink(ev)
		}
	}
	agent.Relay(ctx, ch, relaySink)

	if persist {
		_ = s.mem.AppendAssistant(ctx, in.ConversationID, acc.content.String(), acc.reasoning.String(), acc.calls, acc.truncated)
	}
	return nil
}

// accumulator captures a turn's assistant output to persist after streaming.
type accumulator struct {
	content   strings.Builder
	reasoning strings.Builder
	calls     []models.AIToolCallRecord
	truncated bool
}

func (a *accumulator) add(ev engine.StreamEvent) {
	switch ev.Type {
	case engine.EventTextDelta:
		a.content.WriteString(ev.Text)
	case engine.EventReasoningDelta:
		a.reasoning.WriteString(ev.Text)
	case engine.EventToolCall:
		a.calls = append(a.calls, models.AIToolCallRecord{ID: ev.ToolID, Name: ev.ToolName, Input: ev.Input})
	case engine.EventToolResult:
		for i := range a.calls {
			if a.calls[i].ID == ev.ToolID {
				a.calls[i].Output = ev.Output
				a.calls[i].Err = ev.Err
				break
			}
		}
	case engine.EventDone:
		if ev.Truncated {
			a.truncated = true
		}
	}
}

// AllowedRisks maps a connection's AI mode + destructive opt-in to the tool risk
// tiers the agent may use. Privileged is never allowed; streaming routes are
// excluded by the tools layer.
func AllowedRisks(mode string, allowDestructive bool) map[plugin.RiskLevel]bool {
	allowed := map[plugin.RiskLevel]bool{plugin.RiskSafe: true}
	if mode == "read_write" {
		allowed[plugin.RiskWrite] = true
		if allowDestructive {
			allowed[plugin.RiskDestructive] = true
		}
	}
	return allowed
}

func (s *Service) resolveProvider(ctx context.Context, user models.User, scope Scope) (engine.Provider, string, error) {
	if scope.ProviderID != "" {
		cfg, key, err := s.providers.Resolve(ctx, user.ID, scope.ProviderID)
		if err != nil {
			return nil, "", err
		}
		p, err := s.factory(ctx, cfg.Kind, key, cfg.BaseURL, cfg.DefaultModel)
		return p, cfg.DefaultModel, err
	}
	if s.global.Configured() {
		p, err := s.factory(ctx, models.AIProviderKind(s.global.Kind), s.global.APIKey, s.global.BaseURL, s.global.DefaultModel)
		return p, s.global.DefaultModel, err
	}
	return nil, "", ErrNotConfigured
}

// buildProvider maps a provider kind to an engine adapter. OpenAI and
// OpenAI-compatible run through eino today; other vendors arrive in a later phase.
func buildProvider(ctx context.Context, kind models.AIProviderKind, key, baseURL, model string) (engine.Provider, error) {
	switch kind {
	case models.AIProviderOpenAI, models.AIProviderOpenAICompat:
		return einoadapter.NewOpenAI(ctx, einoadapter.Config{APIKey: key, BaseURL: baseURL, Model: model})
	default:
		return nil, ErrProviderUnsupported
	}
}

// Configured reports whether any provider (user or global) could serve a turn for
// the user. Used by transport to gate the chat endpoint.
func (s *Service) Configured(ctx context.Context, userID string) bool {
	if s.global.Configured() {
		return true
	}
	list, err := s.providers.List(ctx, userID)
	return err == nil && len(list) > 0
}
