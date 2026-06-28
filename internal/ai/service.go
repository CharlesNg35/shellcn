// Package ai coordinates provider resolution, route tools, memory, and streaming turns.
package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/ai/agent"
	"github.com/charlesng35/shellcn/internal/ai/budget"
	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	einoadapter "github.com/charlesng35/shellcn/internal/ai/engine/eino"
	"github.com/charlesng35/shellcn/internal/ai/memory"
	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const defaultMaxSteps = 12

var titleGenerationTimeout = 8 * time.Second

var (
	// ErrNotConfigured means neither a user provider nor the shared config is usable.
	ErrNotConfigured = errors.New("ai: no provider configured")
	// ErrProviderUnsupported means the resolved provider kind has no adapter yet.
	ErrProviderUnsupported = errors.New("ai: provider kind not supported")
	// ErrDisabled means AI is disabled for this connection.
	ErrDisabled = errors.New("ai: disabled for this connection")
)

// ProviderFactory builds an engine.Provider for a resolved config.
type ProviderFactory func(ctx context.Context, kind models.AIProviderKind, key, baseURL, model string) (engine.Provider, error)

// Service is the public AI surface used by transport.
type Service struct {
	providers *aiconfig.Service
	global    config.AIConfig
	routes    tools.RouteSource
	invoker   tools.Invoker
	mem       *memory.Store
	models    *modelreg.Registry
	factory   ProviderFactory
}

// New wires the config service, global config, route source, secure invoker,
// conversation memory, and the model-limit registry.
func New(providers *aiconfig.Service, global config.AIConfig, routes tools.RouteSource, invoker tools.Invoker, mem *memory.Store, models *modelreg.Registry) *Service {
	if models == nil {
		models = modelreg.New()
	}
	return &Service{providers: providers, global: global, routes: routes, invoker: invoker, mem: mem, models: models, factory: buildProvider}
}

// Conversations exposes the conversation/message store for transport CRUD.
func (s *Service) Conversations() *memory.Store { return s.mem }

// WithProviderFactory overrides the engine adapter (used in tests).
func (s *Service) WithProviderFactory(f ProviderFactory) *Service {
	s.factory = f
	return s
}

// Scope selects the provider for a turn. Empty ProviderID means shared AI.
type Scope struct {
	ProviderID string
}

// RunInput is one chat turn's request.
type RunInput struct {
	User             models.User
	ConnID           string
	Protocol         string
	ConnectionTitle  string
	AIMode           models.AIMode
	AllowDestructive bool
	Scope            Scope
	ConversationID   string // when set (with memory wired), history is persisted
	History          []engine.Message
	UserMessage      string
	WorkspaceQuery   string
	RecentOps        []string
	Confirm          tools.Confirmer
}

// Run executes one turn and relays every event to sink. The caller cancels ctx
// to stop the turn cleanly.
func (s *Service) Run(ctx context.Context, in RunInput, sink func(engine.StreamEvent)) error {
	if in.AIMode == models.AIModeDisabled {
		return ErrDisabled
	}

	provider, model, kind, err := s.resolveProvider(ctx, in.User, in.Scope)
	if err != nil {
		return err
	}

	allowed := AllowedRisks(in.AIMode, in.AllowDestructive)
	toolset, err := tools.Build(s.routes, in.Protocol, allowed, s.invoker, in.User, in.ConnID)
	if err != nil {
		return err
	}
	if in.Confirm != nil {
		toolset.WithConfirmer(in.Confirm)
	}

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
	protocolTitle, protocolDescription := s.protocolInfo(in.Protocol)
	system := agent.SystemPrompt(agent.PromptInput{
		ConnectionTitle:     in.ConnectionTitle,
		Protocol:            in.Protocol,
		ProtocolTitle:       protocolTitle,
		ProtocolDescription: protocolDescription,
		AIMode:              in.AIMode,
		Tools:               names,
		WorkspaceQuery:      in.WorkspaceQuery,
		RecentOps:           in.RecentOps,
		HasSubagent:         hasSubagent,
	})

	limits := budget.Limits{ContextWindow: s.models.ContextWindow(ctx, model, registryProvider(kind))}
	if lk, ok := s.models.Lookup(ctx, model, registryProvider(kind)); ok {
		limits.MaxOutputTokens = lk.MaxOutputTokens
	}
	overheadTokens := budget.Estimate(system) + budget.MeasureToolTokens(specs)
	historyBudget := budget.HistoryBudget(limits, overheadTokens, 0)
	maxOut := budget.ResolveOutputTokens(limits, overheadTokens, 0)

	var msgs []engine.Message
	persist := s.mem != nil && in.ConversationID != ""
	if persist {
		if err := s.mem.AppendUser(ctx, in.ConversationID, in.UserMessage); err != nil {
			return err
		}
		summary, history, err := s.mem.History(ctx, in.ConversationID, historyBudget)
		if err != nil {
			return err
		}
		msgs = history
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
		MaxOutTokens: maxOut,
	}, exec)
	if err != nil {
		return err
	}

	acc := &accumulator{}
	relaySink := func(ev engine.StreamEvent) {
		acc.add(ev)
		sink(ev)
	}
	agent.Relay(ctx, ch, relaySink)
	if acc.err != "" && !acc.done {
		sink(engine.StreamEvent{Type: engine.EventDone})
	}

	if persist && acc.err == "" {
		_ = s.mem.AppendAssistant(ctx, in.ConversationID, acc.content.String(), acc.reasoning.String(), acc.calls, acc.truncated)
		s.autoTitle(ctx, provider, model, in.User.ID, in.ConversationID)
	}
	return nil
}

func (s *Service) protocolInfo(protocol string) (string, string) {
	if s.routes == nil {
		return "", ""
	}
	plg, ok := s.routes.Get(protocol)
	if !ok {
		return "", ""
	}
	m := plg.Manifest()
	return strings.TrimSpace(m.Title), strings.TrimSpace(m.Description)
}

// autoTitle tries to resolve the placeholder title once there is enough context.
func (s *Service) autoTitle(ctx context.Context, provider engine.Provider, model, ownerID, convID string) {
	conv, err := s.mem.Get(ctx, ownerID, convID)
	if err != nil || !memory.CanAutoTitle(conv) || s.mem.MessageCount(ctx, convID) <= 2 {
		return
	}
	messages, err := s.mem.Messages(ctx, ownerID, convID)
	if err != nil {
		return
	}
	if title := generateTitle(ctx, provider, model, titleContext(messages)); title != "" {
		s.mem.SetAutoTitle(ctx, convID, title)
	}
}

func generateTitle(ctx context.Context, provider engine.Provider, model, conversation string) string {
	titleCtx, cancel := context.WithTimeout(ctx, titleGenerationTimeout)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		done <- agent.GenerateTitle(titleCtx, provider, model, conversation)
	}()

	select {
	case title := <-done:
		return title
	case <-titleCtx.Done():
		return ""
	}
}

func titleContext(messages []models.AIMessage) string {
	var b strings.Builder
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(content)
	}
	return b.String()
}

// accumulator captures a turn's assistant output to persist after streaming.
type accumulator struct {
	content   strings.Builder
	reasoning strings.Builder
	calls     []models.AIToolCallRecord
	truncated bool
	done      bool
	err       string
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
		a.done = true
	case engine.EventError:
		a.err = ev.Err
	}
}

// AllowedRisks maps a connection's AI mode + destructive opt-in to allowed tool risks.
func AllowedRisks(mode models.AIMode, allowDestructive bool) map[plugin.RiskLevel]bool {
	switch mode {
	case "", models.AIModeReadOnly:
		return map[plugin.RiskLevel]bool{plugin.RiskSafe: true}
	case models.AIModeReadWrite:
		allowed := map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}
		if allowDestructive {
			allowed[plugin.RiskDestructive] = true
		}
		return allowed
	default:
		return map[plugin.RiskLevel]bool{}
	}
}

func (s *Service) resolveProvider(ctx context.Context, user models.User, scope Scope) (engine.Provider, string, models.AIProviderKind, error) {
	if scope.ProviderID != "" {
		cfg, key, err := s.providers.Resolve(ctx, user.ID, scope.ProviderID)
		if err != nil {
			return nil, "", "", err
		}
		p, err := s.factory(ctx, cfg.Kind, key, cfg.BaseURL, cfg.Model)
		return p, cfg.Model, cfg.Kind, err
	}
	if s.global.Configured() {
		kind, ok := aiconfig.SupportedKind(s.global.Kind)
		if !ok {
			return nil, "", "", ErrNotConfigured
		}
		p, err := s.factory(ctx, kind, s.global.APIKey, s.global.BaseURL, s.global.Model)
		return p, s.global.Model, kind, err
	}
	return nil, "", "", ErrNotConfigured
}

func registryProvider(kind models.AIProviderKind) string {
	switch kind {
	case models.AIProviderOpenAI:
		return "openai"
	case models.AIProviderOpenRouter:
		return "openrouter"
	case models.AIProviderAnthropic:
		return "anthropic"
	case models.AIProviderGoogle:
		return "google"
	default:
		return ""
	}
}

// buildProvider maps a provider kind to its engine adapter.
func buildProvider(ctx context.Context, kind models.AIProviderKind, key, baseURL, model string) (engine.Provider, error) {
	cfg := einoadapter.Config{APIKey: key, BaseURL: baseURL, Model: model}
	switch kind {
	case models.AIProviderOpenRouter:
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://openrouter.ai/api/v1"
		}
		return einoadapter.NewOpenAI(ctx, cfg)
	case models.AIProviderOpenAI, models.AIProviderOpenAICompat:
		return einoadapter.NewOpenAI(ctx, cfg)
	case models.AIProviderAnthropic:
		return einoadapter.NewAnthropic(ctx, cfg)
	case models.AIProviderGoogle:
		return einoadapter.NewGoogle(ctx, cfg)
	default:
		return nil, ErrProviderUnsupported
	}
}

// Configured reports whether any provider (user or global) could serve a turn for
// the user. Used by transport to gate the chat endpoint.
func (s *Service) Configured(ctx context.Context, userID string) bool {
	if _, ok := aiconfig.SupportedKind(s.global.Kind); s.global.Configured() && ok {
		return true
	}
	list, err := s.providers.List(ctx, userID)
	return err == nil && len(list) > 0
}
