package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

const (
	subagentMaxSteps     = 15
	subagentMaxOutTokens = 2048
)

type toolExecutor interface {
	engine.ToolExecutor
	Specs() []engine.ToolSpec
}

// Subagent runs a nested, read-only turn and returns a concise summary.
type Subagent struct {
	name     string
	provider engine.Provider
	model    string
	tools    toolExecutor
	protocol string
}

// NewSubagent builds the investigation subagent over a read-only tool set.
func NewSubagent(name string, provider engine.Provider, model string, ro toolExecutor, protocol string) *Subagent {
	return &Subagent{name: name, provider: provider, model: model, tools: ro, protocol: protocol}
}

// Spec is the tool the parent model calls to delegate a read-only investigation.
func (sa *Subagent) Spec() engine.ToolSpec {
	return engine.ToolSpec{
		Name: sa.name,
		Description: "Delegate a focused, multi-step read-only investigation of this " +
			sa.protocol + " connection to a subagent. It explores using read tools and returns a concise summary. " +
			"Use it to gather context without cluttering the conversation.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{"type": "string", "description": "what to investigate and report back"},
			},
			"required": []string{"task"},
		},
	}
}

// Execute runs the nested turn.
func (sa *Subagent) Execute(ctx context.Context, call engine.ToolCall) (any, error) {
	task, _ := call.Input["task"].(string)
	if strings.TrimSpace(task) == "" {
		return nil, fmt.Errorf("investigate: a task is required")
	}
	emit := engine.Progress(ctx)

	system := "You are a read-only investigation subagent for a " + sa.protocol +
		" connection. Use the available read tools to gather what is asked, then reply with a concise, factual summary. " +
		"Do not ask questions; you cannot modify anything."

	ch, err := sa.provider.Stream(ctx, engine.ChatRequest{
		Model:        sa.model,
		System:       system,
		Messages:     []engine.Message{{Role: engine.RoleUser, Content: task}},
		Tools:        sa.tools.Specs(),
		MaxSteps:     subagentMaxSteps,
		MaxOutTokens: subagentMaxOutTokens,
	}, sa.tools)
	if err != nil {
		return nil, err
	}

	var summary strings.Builder
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			summary.WriteString(ev.Text)
		case engine.EventToolCall, engine.EventToolResult:
			ev.Subagent = sa.name
			emit(ev)
		case engine.EventError:
			if summary.Len() == 0 {
				return nil, fmt.Errorf("investigate: %s", ev.Err)
			}
		}
	}
	out := strings.TrimSpace(summary.String())
	if out == "" {
		out = "(the investigation produced no findings)"
	}
	return out, nil
}

// Composite dispatches tool calls to either route tools or subagents.
type Composite struct {
	base      toolExecutor
	subagents map[string]*Subagent
}

// NewComposite combines the route tool set with subagents.
func NewComposite(base toolExecutor, subagents ...*Subagent) *Composite {
	m := make(map[string]*Subagent, len(subagents))
	for _, sa := range subagents {
		m[sa.name] = sa
	}
	return &Composite{base: base, subagents: m}
}

// Specs is the full catalogue (route tools + subagents).
func (c *Composite) Specs() []engine.ToolSpec {
	specs := append([]engine.ToolSpec{}, c.base.Specs()...)
	for _, sa := range c.subagents {
		specs = append(specs, sa.Spec())
	}
	return specs
}

// Execute routes to the matching subagent, else to the base tool set.
func (c *Composite) Execute(ctx context.Context, call engine.ToolCall) (any, error) {
	if sa, ok := c.subagents[call.Name]; ok {
		return sa.Execute(ctx, call)
	}
	return c.base.Execute(ctx, call)
}
