package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

type titleProvider struct {
	gotMaxOut int
}

func (p *titleProvider) Models(context.Context) ([]engine.ModelInfo, error) { return nil, nil }

func (p *titleProvider) Stream(_ context.Context, req engine.ChatRequest, _ engine.ToolExecutor) (<-chan engine.StreamEvent, error) {
	p.gotMaxOut = req.MaxOutTokens
	out := make(chan engine.StreamEvent, 4)
	go func() {
		defer close(out)
		if req.MaxOutTokens < 512 {
			out <- engine.StreamEvent{Type: engine.EventDone, Truncated: true}
			return
		}
		out <- engine.StreamEvent{Type: engine.EventReasoningDelta, Text: "thinking"}
		out <- engine.StreamEvent{Type: engine.EventTextDelta, Text: "Database Backup Failure"}
		out <- engine.StreamEvent{Type: engine.EventDone}
	}()
	return out, nil
}

func TestGenerateTitleAllowsReasoningBudget(t *testing.T) {
	provider := &titleProvider{}

	title := GenerateTitle(context.Background(), provider, "reasoning-model", "user: why did backup fail?\nassistant: The backup failed because the database disk is full.")

	if title != "Database Backup Failure" {
		t.Fatalf("unexpected title %q", title)
	}
	if provider.gotMaxOut < 512 {
		t.Fatalf("title generation budget too low: %d", provider.gotMaxOut)
	}
}

func TestCleanTitleIsRuneSafe(t *testing.T) {
	title := cleanTitle(strings.Repeat("é", 90))
	if len([]rune(title)) != 80 {
		t.Fatalf("title should be trimmed to 80 runes, got %d", len([]rune(title)))
	}
}

func TestCleanTitleStripsModelArtifacts(t *testing.T) {
	title := cleanTitle(" \n- Title:  Database   Backup Failure.\nextra explanation")
	if title != "Database Backup Failure" {
		t.Fatalf("unexpected title %q", title)
	}
}
