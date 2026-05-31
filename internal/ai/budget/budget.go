// Package budget computes a turn's token budgeting. The history budget is the
// model's context window minus the system prompt, the tool schemas, a safety
// margin, and the reserved output tokens, clamped to sane bounds. Token counts
// use a deterministic heuristic (Estimate); a real tokenizer can replace it
// behind the same function without touching callers.
package budget

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

const (
	safetyMargin = 512

	DefaultHistoryBudget = 32_000
	MinHistoryBudget     = 4_000
	MaxHistoryBudget     = 200_000

	MaxOutputTokens   = 8_192
	MinOutputTokens   = 2_048
	MinContextWindow  = 16_000
	charsPerTokenWord = 4
)

// Limits are a model's known token limits (0 = unknown).
type Limits struct {
	ContextWindow   int
	MaxOutputTokens int
}

// Overhead is the non-history token cost of a turn (system prompt + tool schemas
// + a fixed safety margin).
func Overhead(systemTokens, toolTokens int) int {
	return systemTokens + toolTokens + safetyMargin
}

// ResolveOutputTokens decides how many output tokens to reserve given the model's
// limits and the turn's overhead.
func ResolveOutputTokens(limits Limits, systemTokens, toolTokens int) int {
	capTok := max(0, limits.MaxOutputTokens)
	if limits.ContextWindow <= 0 {
		if capTok > 0 {
			return min(MaxOutputTokens, capTok)
		}
		return MaxOutputTokens
	}
	available := limits.ContextWindow - Overhead(systemTokens, toolTokens) - MinHistoryBudget
	resolved := MaxOutputTokens
	if available < MaxOutputTokens {
		resolved = max(MinOutputTokens, available)
	}
	if capTok > 0 {
		return max(1, min(resolved, capTok))
	}
	return resolved
}

// HistoryBudget is the token budget available for conversation history: the
// context window minus overhead and the reserved output, clamped to bounds. With
// an unknown window it falls back to a safe default.
func HistoryBudget(limits Limits, systemTokens, toolTokens int) int {
	if limits.ContextWindow <= 0 {
		return DefaultHistoryBudget
	}
	out := ResolveOutputTokens(limits, systemTokens, toolTokens)
	b := limits.ContextWindow - Overhead(systemTokens, toolTokens) - out
	return max(MinHistoryBudget, min(b, MaxHistoryBudget))
}

// Estimate approximates the token count of a string with a deterministic
// char/word blend (~4 chars or ~0.75 words per token). It is intentionally cheap
// and stable for tests; a real tokenizer can replace it without changing callers.
func Estimate(text string) int {
	if text == "" {
		return 0
	}
	chars := len([]rune(text))
	words := len(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r)
	}))
	byChars := (chars + charsPerTokenWord - 1) / charsPerTokenWord
	byWords := (words*4 + 2) / 3 // ~1.33 tokens/word
	if byWords > byChars {
		return byWords
	}
	if byChars == 0 {
		return 1
	}
	return byChars
}

// MeasureToolTokens estimates the prompt cost of the tool catalogue from each
// tool's name, description, and JSON-schema parameters.
func MeasureToolTokens(tools []engine.ToolSpec) int {
	if len(tools) == 0 {
		return 0
	}
	var b strings.Builder
	for i, t := range tools {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t.Name)
		b.WriteByte(':')
		b.WriteString(t.Description)
		b.WriteByte(':')
		if t.Parameters != nil {
			if raw, err := json.Marshal(t.Parameters); err == nil {
				b.Write(raw)
			}
		}
	}
	return Estimate(b.String())
}
