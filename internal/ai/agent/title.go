package agent

import (
	"context"
	"strings"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

const titleMaxOutTokens = 1024

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
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			s = line
			break
		}
	}
	s = strings.TrimLeft(s, "-*# \t")
	for _, prefix := range []string{"conversation title:", "title:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, "`\"'. ")
	runes := []rune(s)
	if len(runes) < 2 {
		return ""
	}
	if len(runes) > 80 {
		return string(runes[:80])
	}
	return s
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
