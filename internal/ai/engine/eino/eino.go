// Package eino is the ONLY package that imports cloudwego/eino + eino-ext. It
// adapts the framework to ShellCN's engine.Provider seam, running an explicit
// tool-calling loop so each tool invocation flows through our risk-gated
// executor. The rest of internal/ai depends only on engine interfaces, so the
// framework stays a swappable implementation detail.
package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"google.golang.org/genai"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

// anthropicMaxTokens is the required max_tokens Anthropic needs per request.
const anthropicMaxTokens = 8192

// Config is the minimal provider wiring the adapter needs.
type Config struct {
	APIKey  string
	BaseURL string // empty for the vendor default; set for openai-compatible endpoints
	Model   string
}

// Provider is an eino-backed engine.Provider for OpenAI / OpenAI-compatible
// endpoints.
type Provider struct {
	cm    model.ToolCallingChatModel
	model string
}

// NewOpenAI builds an OpenAI (or OpenAI-compatible) provider. A BaseURL points
// it at Ollama, OpenRouter, vLLM, gateways, etc.
func NewOpenAI(ctx context.Context, cfg Config) (*Provider, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("eino openai: %w", err)
	}
	return &Provider{cm: cm, model: cfg.Model}, nil
}

// NewAnthropic builds a Claude provider.
func NewAnthropic(ctx context.Context, cfg Config) (*Provider, error) {
	c := &claude.Config{APIKey: cfg.APIKey, Model: cfg.Model, MaxTokens: anthropicMaxTokens}
	if cfg.BaseURL != "" {
		c.BaseURL = &cfg.BaseURL
	}
	cm, err := claude.NewChatModel(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("eino claude: %w", err)
	}
	return &Provider{cm: cm, model: cfg.Model}, nil
}

// NewGoogle builds a Gemini provider.
func NewGoogle(ctx context.Context, cfg Config) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: cfg.APIKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, fmt.Errorf("eino gemini client: %w", err)
	}
	cm, err := gemini.NewChatModel(ctx, &gemini.Config{Client: client, Model: cfg.Model})
	if err != nil {
		return nil, fmt.Errorf("eino gemini: %w", err)
	}
	return &Provider{cm: cm, model: cfg.Model}, nil
}

// Models reports the configured model. A live catalogue query is not exposed by
// the adapter; the model allow-list is managed in provider config.
func (p *Provider) Models(_ context.Context) ([]engine.ModelInfo, error) {
	return []engine.ModelInfo{{ID: p.model}}, nil
}

// Stream runs one turn (with an internal ReAct loop over tool calls) and emits
// engine.StreamEvents until the channel closes. The caller cancels ctx to stop.
func (p *Provider) Stream(ctx context.Context, req engine.ChatRequest, exec engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	cm := p.cm
	if len(req.Tools) > 0 {
		infos, err := toolInfos(req.Tools)
		if err != nil {
			return nil, err
		}
		cm, err = p.cm.WithTools(infos)
		if err != nil {
			return nil, err
		}
	}

	out := make(chan engine.StreamEvent, 64)
	go func() {
		defer close(out)
		p.runTurn(ctx, cm, req, exec, out)
	}()
	return out, nil
}

func (p *Provider) runTurn(ctx context.Context, cm model.ToolCallingChatModel, req engine.ChatRequest, exec engine.ToolExecutor, out chan<- engine.StreamEvent) {
	msgs := buildMessages(req.System, req.Messages)

	var opts []model.Option
	if req.MaxOutTokens > 0 {
		opts = append(opts, model.WithMaxTokens(req.MaxOutTokens))
	}

	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 1
	}

	for step := 0; step < maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			out <- engine.StreamEvent{Type: engine.EventError, Err: err.Error()}
			return
		}

		sr, err := cm.Stream(ctx, msgs, opts...)
		if err != nil {
			out <- engine.StreamEvent{Type: engine.EventError, Err: err.Error()}
			return
		}
		chunks, ferr := drain(sr, out)
		if ferr != nil {
			out <- engine.StreamEvent{Type: engine.EventError, Err: ferr.Error()}
			return
		}
		full, err := schema.ConcatMessages(chunks)
		if err != nil {
			out <- engine.StreamEvent{Type: engine.EventError, Err: err.Error()}
			return
		}

		if len(full.ToolCalls) == 0 {
			truncated := full.ResponseMeta != nil && full.ResponseMeta.FinishReason == "length"
			out <- engine.StreamEvent{Type: engine.EventDone, Usage: usageOf(full), Truncated: truncated}
			return
		}

		msgs = append(msgs, full)
		// A tool may stream intermediate progress (e.g. a subagent's nested tool
		// calls) onto this same turn stream. Execute runs synchronously in this
		// goroutine, so writing to out from the emitter stays ordered and race-free.
		toolCtx := engine.WithProgress(ctx, func(ev engine.StreamEvent) { out <- ev })
		for _, tc := range full.ToolCalls {
			input := parseArgs(tc.Function.Arguments)
			out <- engine.StreamEvent{Type: engine.EventToolCall, ToolID: tc.ID, ToolName: tc.Function.Name, Input: input}

			result, execErr := exec.Execute(toolCtx, engine.ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: input})
			if execErr != nil {
				out <- engine.StreamEvent{Type: engine.EventToolResult, ToolID: tc.ID, ToolName: tc.Function.Name, Err: execErr.Error()}
				msgs = append(msgs, schema.ToolMessage("error: "+execErr.Error(), tc.ID))
				continue
			}
			out <- engine.StreamEvent{Type: engine.EventToolResult, ToolID: tc.ID, ToolName: tc.Function.Name, Output: result}
			msgs = append(msgs, schema.ToolMessage(stringify(result), tc.ID))
		}
		out <- engine.StreamEvent{Type: engine.EventStep}
	}

	// Step budget exhausted with tool calls still pending: report a capped done.
	out <- engine.StreamEvent{Type: engine.EventDone, Truncated: true}
}

// drain reads a model stream to EOF, emitting text/reasoning deltas, and returns
// the collected chunks for tool-call merging.
func drain(sr *schema.StreamReader[*schema.Message], out chan<- engine.StreamEvent) ([]*schema.Message, error) {
	defer sr.Close()
	var chunks []*schema.Message
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return chunks, nil
		}
		if err != nil {
			return nil, err
		}
		if chunk.Content != "" {
			out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: chunk.Content}
		}
		if chunk.ReasoningContent != "" {
			out <- engine.StreamEvent{Type: engine.EventReasoningDelta, Text: chunk.ReasoningContent}
		}
		chunks = append(chunks, chunk)
	}
}

func buildMessages(system string, history []engine.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(history)+1)
	if system != "" {
		out = append(out, schema.SystemMessage(system))
	}
	for _, m := range history {
		switch m.Role {
		case engine.RoleSystem:
			out = append(out, schema.SystemMessage(m.Content))
		case engine.RoleUser:
			out = append(out, schema.UserMessage(m.Content))
		case engine.RoleAssistant:
			out = append(out, schema.AssistantMessage(m.Content, toSchemaToolCalls(m.ToolCalls)))
		case engine.RoleTool:
			out = append(out, schema.ToolMessage(m.Content, m.ToolCallID))
		}
	}
	return out
}

func toSchemaToolCalls(calls []engine.ToolCall) []schema.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]schema.ToolCall, 0, len(calls))
	for _, c := range calls {
		args, _ := json.Marshal(c.Input)
		out = append(out, schema.ToolCall{
			ID:       c.ID,
			Function: schema.FunctionCall{Name: c.Name, Arguments: string(args)},
		})
	}
	return out
}

func toolInfos(specs []engine.ToolSpec) ([]*schema.ToolInfo, error) {
	out := make([]*schema.ToolInfo, 0, len(specs))
	for _, s := range specs {
		info := &schema.ToolInfo{Name: s.Name, Desc: s.Description}
		if len(s.Parameters) > 0 {
			raw, err := json.Marshal(s.Parameters)
			if err != nil {
				return nil, err
			}
			js := &jsonschema.Schema{}
			if err := json.Unmarshal(raw, js); err != nil {
				return nil, fmt.Errorf("tool %q params: %w", s.Name, err)
			}
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(js)
		}
		out = append(out, info)
	}
	return out, nil
}

func parseArgs(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{}
	}
	return m
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func usageOf(m *schema.Message) *engine.Usage {
	if m == nil || m.ResponseMeta == nil || m.ResponseMeta.Usage == nil {
		return nil
	}
	u := m.ResponseMeta.Usage
	return &engine.Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
}
