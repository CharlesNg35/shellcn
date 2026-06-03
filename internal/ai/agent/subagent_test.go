package agent_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai/agent"
	"github.com/charlesng35/shellcn/internal/ai/engine"
)

// fakeProvider scripts a nested subagent run: it calls the first read tool, then
// answers with a summary.
type fakeProvider struct{}

func (fakeProvider) Models(context.Context) ([]engine.ModelInfo, error) { return nil, nil }

func (fakeProvider) Stream(ctx context.Context, req engine.ChatRequest, exec engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	out := make(chan engine.StreamEvent, 8)
	go func() {
		defer close(out)
		if len(req.Tools) > 0 && exec != nil {
			call := engine.ToolCall{ID: "n1", Name: req.Tools[0].Name, Input: map[string]any{}}
			out <- engine.StreamEvent{Type: engine.EventToolCall, ToolID: call.ID, ToolName: call.Name}
			_, _ = exec.Execute(ctx, call)
			out <- engine.StreamEvent{Type: engine.EventToolResult, ToolID: call.ID, ToolName: call.Name}
		}
		out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: "found 3 containers"}
		out <- engine.StreamEvent{Type: engine.EventDone}
	}()
	return out, nil
}

// roTools is a minimal read-only tool executor.
type roTools struct{ executed []string }

func (r *roTools) Specs() []engine.ToolSpec {
	return []engine.ToolSpec{{Name: "list_containers", Description: "list", Parameters: map[string]any{"type": "object"}}}
}

func (r *roTools) Execute(_ context.Context, call engine.ToolCall) (any, error) {
	r.executed = append(r.executed, call.Name)
	return map[string]any{"ok": true}, nil
}

func TestSubagentReturnsSummaryAndStreamsPrefixedProgress(t *testing.T) {
	ro := &roTools{}
	sa := agent.NewSubagent("investigate", fakeProvider{}, "gpt-4o", ro, "docker")

	var nested []engine.StreamEvent
	ctx := engine.WithProgress(context.Background(), func(ev engine.StreamEvent) { nested = append(nested, ev) })

	out, err := sa.Execute(ctx, engine.ToolCall{Name: "investigate", Input: map[string]any{"task": "what is running"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	summary, _ := out.(string)
	if !strings.Contains(summary, "found 3 containers") {
		t.Fatalf("subagent should return its text as a summary, got %q", summary)
	}

	// The nested read tool ran, and its progress was re-emitted with the prefix.
	if len(ro.executed) != 1 || ro.executed[0] != "list_containers" {
		t.Fatalf("nested tool not executed: %v", ro.executed)
	}
	var sawPrefixed bool
	for _, ev := range nested {
		if ev.Subagent == "investigate" && (ev.Type == engine.EventToolCall || ev.Type == engine.EventToolResult) {
			sawPrefixed = true
		}
	}
	if !sawPrefixed {
		t.Fatalf("nested progress not streamed with subagent prefix: %+v", nested)
	}
}

func TestSubagentRequiresTask(t *testing.T) {
	sa := agent.NewSubagent("investigate", fakeProvider{}, "gpt-4o", &roTools{}, "docker")
	if _, err := sa.Execute(context.Background(), engine.ToolCall{Name: "investigate", Input: map[string]any{}}); err == nil {
		t.Fatal("missing task should error")
	}
}

func TestCompositeRoutesToSubagentOrBase(t *testing.T) {
	base := &roTools{}
	sa := agent.NewSubagent("investigate", fakeProvider{}, "gpt-4o", &roTools{}, "docker")
	comp := agent.NewComposite(base, sa)

	// Catalogue includes base tools + the subagent.
	names := map[string]bool{}
	for _, s := range comp.Specs() {
		names[s.Name] = true
	}
	if !names["list_containers"] || !names["investigate"] {
		t.Fatalf("composite catalogue wrong: %v", names)
	}

	// A base-tool call routes to the base executor.
	if _, err := comp.Execute(context.Background(), engine.ToolCall{Name: "list_containers"}); err != nil {
		t.Fatalf("base execute: %v", err)
	}
	if len(base.executed) != 1 {
		t.Fatalf("base tool should have run, got %v", base.executed)
	}
}

func TestSystemPromptIncludesRecentOpsAndSubagent(t *testing.T) {
	prompt := agent.SystemPrompt(agent.PromptInput{
		ConnectionTitle: "prod", Protocol: "docker", AIMode: "read_only",
		Tools:       []string{"list_containers"},
		HasSubagent: true,
		RecentOps:   []string{"error docker.container.delete: permission denied"},
	})
	if !strings.Contains(prompt, "investigate") {
		t.Fatal("prompt should mention the subagent")
	}
	if !strings.Contains(prompt, "recent operations") {
		t.Fatalf("prompt should include recent operations section:\n%s", prompt)
	}
	if !strings.Contains(prompt, "permission denied") {
		t.Fatal("prompt should surface the failed operation")
	}
}
