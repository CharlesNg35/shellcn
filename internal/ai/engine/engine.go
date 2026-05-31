// Package engine is the framework-agnostic seam between ShellCN and whatever
// LLM/agent library backs it. The rest of internal/ai depends only on these
// small interfaces; the concrete implementation (eino) lives in engine/eino and
// is imported nowhere else, so the framework is a swappable detail.
package engine

import "context"

type progressKey struct{}

// WithProgress attaches an emitter that a tool executor may use to stream
// intermediate events (e.g. a subagent's nested tool progress) into the running
// turn. The provider sets this on the context it passes to ToolExecutor.Execute,
// so emits land on the same event stream the turn is already relaying.
func WithProgress(ctx context.Context, emit func(StreamEvent)) context.Context {
	return context.WithValue(ctx, progressKey{}, emit)
}

// Progress returns the emitter attached by WithProgress, or a no-op.
func Progress(ctx context.Context) func(StreamEvent) {
	if emit, ok := ctx.Value(progressKey{}).(func(StreamEvent)); ok && emit != nil {
		return emit
	}
	return func(StreamEvent) {}
}

// EventType tags a streamed turn event.
type EventType string

const (
	EventTextDelta      EventType = "text_delta"
	EventReasoningDelta EventType = "reasoning_delta"
	EventToolCall       EventType = "tool_call"
	EventToolResult     EventType = "tool_result"
	EventStep           EventType = "step"
	EventError          EventType = "error"
	EventDone           EventType = "done"
)

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is a model-requested invocation of a tool.
type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// Message is one entry of conversation history passed to the provider.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	// ToolCalls is set on assistant messages that called tools.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID + Name identify a tool-result message.
	ToolCallID string `json:"toolCallId,omitempty"`
	Name       string `json:"name,omitempty"`
}

// ToolSpec describes a tool to the model. Parameters is a JSON Schema object.
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Usage carries token accounting reported on a done event.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// StreamEvent is one event relayed from a running turn.
type StreamEvent struct {
	Type     EventType      `json:"type"`
	Text     string         `json:"text,omitempty"`
	ToolName string         `json:"toolName,omitempty"`
	ToolID   string         `json:"toolId,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
	Output   any            `json:"output,omitempty"`
	Err      string         `json:"err,omitempty"`
	Usage    *Usage         `json:"usage,omitempty"`
	// Subagent, when set, marks a tool event as nested work performed by a
	// subagent (the UI prefixes it and styles it distinctly).
	Subagent  string `json:"subagent,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// ChatRequest is one provider turn: system prompt, history, tools, and caps.
type ChatRequest struct {
	Model        string
	System       string
	Messages     []Message
	Tools        []ToolSpec
	MaxSteps     int
	MaxOutTokens int
}

// ToolExecutor runs a model-requested tool call and returns its result. The
// tools package implements this so the engine never learns about routes/security.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (any, error)
}

// ModelInfo describes a model the provider exposes.
type ModelInfo struct {
	ID            string `json:"id"`
	ContextWindow int    `json:"contextWindow,omitempty"`
}

// Provider is a configured LLM endpoint capable of streaming a tool-calling turn.
type Provider interface {
	// Models lists the provider's available models (for the switcher/validation).
	Models(ctx context.Context) ([]ModelInfo, error)
	// Stream runs one turn, invoking exec for tool calls and emitting events on
	// the returned channel until it closes with a done (or error) event.
	Stream(ctx context.Context, req ChatRequest, exec ToolExecutor) (<-chan StreamEvent, error)
}
