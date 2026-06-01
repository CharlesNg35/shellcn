package eino

import (
	"context"
	"os"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai/engine"
)

// toolInfos is internal; this test pins the JSON-schema → eino ToolInfo
// conversion (the error-prone seam) without needing a provider key.
func TestToolInfoConversionRoundTrips(t *testing.T) {
	specs := []engine.ToolSpec{
		{
			Name:        "list_items",
			Description: "Read items",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string", "description": "item name"},
					"limit": map[string]any{"type": "number"},
				},
				"required": []any{"name"},
			},
		},
		{Name: "no_args", Description: "no params"},
	}

	infos, err := toolInfos(specs)
	if err != nil {
		t.Fatalf("toolInfos: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("want 2 tool infos, got %d", len(infos))
	}
	if infos[0].Name != "list_items" || infos[0].ParamsOneOf == nil {
		t.Fatalf("first tool malformed: %+v", infos[0])
	}
	// The params must convert to a valid JSON schema for the model.
	if _, err := infos[0].ToJSONSchema(); err != nil {
		t.Fatalf("ParamsOneOf.ToJSONSchema: %v", err)
	}
	if infos[1].ParamsOneOf != nil {
		t.Fatal("no-arg tool should have nil ParamsOneOf")
	}
}

func TestParseArgs(t *testing.T) {
	if m := parseArgs(""); len(m) != 0 {
		t.Fatalf("empty args should yield empty map, got %v", m)
	}
	if m := parseArgs(`{"a":1}`); m["a"] != float64(1) {
		t.Fatalf("parse failed: %v", m)
	}
	if m := parseArgs("not json"); len(m) != 0 {
		t.Fatalf("bad json should yield empty map, got %v", m)
	}
}

// TestLiveStream exercises a real provider end-to-end. It is env-gated: set
// SHELLCN_AI_TEST_KEY (and optionally SHELLCN_AI_TEST_MODEL / _BASEURL) to run.
func TestLiveStream(t *testing.T) {
	key := os.Getenv("SHELLCN_AI_TEST_KEY")
	if key == "" {
		t.Skip("set SHELLCN_AI_TEST_KEY to run the live provider test")
	}
	mdl := os.Getenv("SHELLCN_AI_TEST_MODEL")
	if mdl == "" {
		mdl = "gpt-4o-mini"
	}
	ctx := context.Background()
	p, err := NewOpenAI(ctx, Config{APIKey: key, BaseURL: os.Getenv("SHELLCN_AI_TEST_BASEURL"), Model: mdl})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	assertPongStream(ctx, t, p)
}

// TestLiveAnthropic / TestLiveGoogle exercise the other adapters; env-gated.
func TestLiveAnthropic(t *testing.T) {
	key := os.Getenv("SHELLCN_AI_TEST_ANTHROPIC_KEY")
	if key == "" {
		t.Skip("set SHELLCN_AI_TEST_ANTHROPIC_KEY to run")
	}
	mdl := os.Getenv("SHELLCN_AI_TEST_ANTHROPIC_MODEL")
	if mdl == "" {
		mdl = "claude-haiku-4-5"
	}
	ctx := context.Background()
	p, err := NewAnthropic(ctx, Config{APIKey: key, Model: mdl})
	if err != nil {
		t.Fatalf("new anthropic: %v", err)
	}
	assertPongStream(ctx, t, p)
}

func TestLiveGoogle(t *testing.T) {
	key := os.Getenv("SHELLCN_AI_TEST_GOOGLE_KEY")
	if key == "" {
		t.Skip("set SHELLCN_AI_TEST_GOOGLE_KEY to run")
	}
	mdl := os.Getenv("SHELLCN_AI_TEST_GOOGLE_MODEL")
	if mdl == "" {
		mdl = "gemini-2.5-flash"
	}
	ctx := context.Background()
	p, err := NewGoogle(ctx, Config{APIKey: key, Model: mdl})
	if err != nil {
		t.Fatalf("new google: %v", err)
	}
	assertPongStream(ctx, t, p)
}

func assertPongStream(ctx context.Context, t *testing.T, p *Provider) {
	t.Helper()
	ch, err := p.Stream(ctx, engine.ChatRequest{
		System:       "You are a test bot. Reply with the single word: pong.",
		Messages:     []engine.Message{{Role: engine.RoleUser, Content: "ping"}},
		MaxSteps:     2,
		MaxOutTokens: 32,
	}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var text string
	var done bool
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			text += ev.Text
		case engine.EventError:
			t.Fatalf("stream error: %s", ev.Err)
		case engine.EventDone:
			done = true
		}
	}
	if !done || text == "" {
		t.Fatalf("expected text + done; text=%q done=%v", text, done)
	}
}
