package budget

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

func TestHistoryBudgetClamped(t *testing.T) {
	if b := HistoryBudget(Limits{}, 100, 100); b != DefaultHistoryBudget {
		t.Fatalf("unknown window should yield default, got %d", b)
	}
	if b := HistoryBudget(Limits{ContextWindow: 2_000_000}, 100, 100); b != MaxHistoryBudget {
		t.Fatalf("large window should clamp to max, got %d", b)
	}
	if b := HistoryBudget(Limits{ContextWindow: 20_000}, 100, 100); b < MinHistoryBudget {
		t.Fatalf("budget below min: %d", b)
	}
	b := HistoryBudget(Limits{ContextWindow: 128_000}, 1000, 500)
	if b <= 0 || b > 128_000 {
		t.Fatalf("unexpected budget %d", b)
	}
}

func TestResolveOutputTokens(t *testing.T) {
	if o := ResolveOutputTokens(Limits{ContextWindow: 128_000}, 100, 100); o != MaxOutputTokens {
		t.Fatalf("want max output, got %d", o)
	}
	if o := ResolveOutputTokens(Limits{ContextWindow: 128_000, MaxOutputTokens: 4096}, 100, 100); o != 4096 {
		t.Fatalf("provider cap should win, got %d", o)
	}
	if o := ResolveOutputTokens(Limits{}, 0, 0); o != MaxOutputTokens {
		t.Fatalf("want default max, got %d", o)
	}
}

func TestEstimateMonotonic(t *testing.T) {
	if Estimate("") != 0 {
		t.Fatal("empty string is 0 tokens")
	}
	short := Estimate("hello world")
	long := Estimate("hello world this is a much longer sentence with more words in it")
	if long <= short {
		t.Fatalf("longer text should estimate more tokens: %d <= %d", long, short)
	}
}

func TestMeasureToolTokens(t *testing.T) {
	none := MeasureToolTokens(nil)
	some := MeasureToolTokens([]engine.ToolSpec{
		{Name: "list", Description: "list things", Parameters: map[string]any{"type": "object"}},
	})
	if none != 0 || some <= 0 {
		t.Fatalf("tool token measurement wrong: none=%d some=%d", none, some)
	}
}
