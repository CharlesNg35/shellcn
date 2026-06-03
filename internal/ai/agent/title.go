package agent

import (
	"context"
	"strings"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

const titleMaxOutTokens = 64

// GenerateTitle asks the model for a short conversation title from the first
// exchange. It returns "" on any failure so the caller can fall back.
func GenerateTitle(ctx context.Context, provider engine.Provider, model, userMessage, assistantReply string) string {
	system := "Generate a short conversation title (max 8 words) capturing the topic. " +
		"Reply with ONLY the title — no quotes, no trailing punctuation."
	prompt := "User: " + truncate(userMessage, 500) + "\nAssistant: " + truncate(assistantReply, 500)

	ch, err := provider.Stream(ctx, engine.ChatRequest{
		Model:        model,
		System:       system,
		Messages:     []engine.Message{{Role: engine.RoleUser, Content: prompt}},
		MaxSteps:     1,
		MaxOutTokens: titleMaxOutTokens,
	}, nil)
	if err != nil {
		return ""
	}

	var b strings.Builder
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			b.WriteString(ev.Text)
		case engine.EventError:
			return ""
		}
	}
	return cleanTitle(b.String())
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `."'`)
	if len(s) > 80 {
		s = s[:80]
	}
	if len([]rune(s)) < 2 {
		return ""
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
