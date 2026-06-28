package aiconfig_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
)

func newService(t *testing.T, global config.AIConfig) (*aiconfig.Service, *store.Store) {
	t.Helper()
	key, _ := secrets.GenerateMasterKey()
	vault, err := secrets.NewVault(key)
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	st := store.NewMemory()
	return aiconfig.New(st.AIProviders, vault, global), st
}

func validInput() aiconfig.Input {
	return aiconfig.Input{
		Kind:   models.AIProviderOpenAI,
		Name:   "My OpenAI",
		APIKey: "sk-secret-123",
		Models: []string{"gpt-4o", "gpt-4o-mini"},
		Model:  "gpt-4o",
	}
}

func TestCreateEncryptsKeyAndNeverReturnsIt(t *testing.T) {
	svc, st := newService(t, config.AIConfig{})
	ctx := context.Background()

	sum, err := svc.Create(ctx, "user-1", validInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !sum.HasKey {
		t.Fatal("summary should report HasKey")
	}

	// The stored ciphertext must not contain the plaintext key.
	row, err := st.AIProviders.Get(ctx, sum.ID)
	if err != nil {
		t.Fatalf("get row: %v", err)
	}
	if len(row.APIKeyCiphertext) == 0 {
		t.Fatal("key not encrypted at rest")
	}
	if bytes.Contains(row.APIKeyCiphertext, []byte("sk-secret-123")) {
		t.Fatal("plaintext key leaked into ciphertext")
	}

	// Resolve round-trips the decrypted key for chat-time use.
	_, plain, err := svc.Resolve(ctx, "user-1", sum.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if plain != "sk-secret-123" {
		t.Fatalf("decrypted key = %q", plain)
	}
}

func TestUpdateEmptyKeyPreservesStored(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()
	sum, err := svc.Create(ctx, "user-1", validInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	in := validInput()
	in.APIKey = "" // keep existing
	in.Name = "Renamed"
	if _, err := svc.Update(ctx, "user-1", sum.ID, in); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, plain, err := svc.Resolve(ctx, "user-1", sum.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if plain != "sk-secret-123" {
		t.Fatalf("key should be preserved, got %q", plain)
	}

	in.APIKey = "sk-rotated"
	if _, err := svc.Update(ctx, "user-1", sum.ID, in); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	_, plain, _ = svc.Resolve(ctx, "user-1", sum.ID)
	if plain != "sk-rotated" {
		t.Fatalf("key should rotate, got %q", plain)
	}
}

func TestUpdateProviderKindRequiresNewKey(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()
	sum, err := svc.Create(ctx, "user-1", validInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	in := validInput()
	in.Kind = models.AIProviderOpenRouter
	in.Name = "OpenRouter"
	in.APIKey = ""
	in.Model = "openai/gpt-4o"
	if _, err := svc.Update(ctx, "user-1", sum.ID, in); !errors.Is(err, aiconfig.ErrInvalid()) {
		t.Fatalf("provider kind change without key: want ErrInvalid, got %v", err)
	}
}

func TestProviderNamesAreUniquePerOwner(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()

	first, err := svc.Create(ctx, "user-1", validInput())
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	dup := validInput()
	dup.Name = "my openai"
	if _, err := svc.Create(ctx, "user-1", dup); !errors.Is(err, models.ErrConflict) {
		t.Fatalf("duplicate same owner: want ErrConflict, got %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", dup); err != nil {
		t.Fatalf("same name for another owner should be allowed: %v", err)
	}

	second := validInput()
	second.Name = "Backup"
	second.Model = "gpt-4o-mini"
	second.Models = []string{"gpt-4o-mini"}
	sum, err := svc.Create(ctx, "user-1", second)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	second.Name = first.Name
	if _, err := svc.Update(ctx, "user-1", sum.ID, second); !errors.Is(err, models.ErrConflict) {
		t.Fatalf("rename duplicate: want ErrConflict, got %v", err)
	}
}

func TestOwnerScopingHidesOthers(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()
	sum, _ := svc.Create(ctx, "user-1", validInput())

	if _, _, err := svc.Resolve(ctx, "user-2", sum.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other owner resolve: want ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, "user-2", sum.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other owner delete: want ErrNotFound, got %v", err)
	}
	list, _ := svc.List(ctx, "user-2")
	if len(list) != 0 {
		t.Fatalf("user-2 should see no providers, got %d", len(list))
	}
}

func TestModelsPerKind(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()

	withList, _ := svc.Create(ctx, "user-1", validInput())
	got, err := svc.Models(ctx, "user-1", withList.ID)
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	if len(got) != 2 || got[0] != "gpt-4o" {
		t.Fatalf("allow-list should win: %v", got)
	}

	noList := aiconfig.Input{Kind: models.AIProviderAnthropic, Name: "Claude", APIKey: "k", Model: "claude-sonnet-4-5"}
	sum, _ := svc.Create(ctx, "user-1", noList)
	got, _ = svc.Models(ctx, "user-1", sum.ID)
	if len(got) == 0 {
		t.Fatal("anthropic should fall back to static defaults")
	}

	openRouter := aiconfig.Input{Kind: models.AIProviderOpenRouter, Name: "OpenRouter", APIKey: "k", Model: "openai/gpt-4o"}
	sum, _ = svc.Create(ctx, "user-1", openRouter)
	got, _ = svc.Models(ctx, "user-1", sum.ID)
	if len(got) == 0 || got[0] != "openai/gpt-4o" {
		t.Fatalf("openrouter should fall back to static defaults: %v", got)
	}
}

func TestDraftProviderTestValidation(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()

	cases := []aiconfig.Input{
		{Kind: "bogus", APIKey: "k", Model: "m"},
		{Kind: models.AIProviderOpenAI, APIKey: "", Model: "m"},
		{Kind: models.AIProviderOpenAI, APIKey: "k", Model: ""},
		{Kind: models.AIProviderOpenAICompat, BaseURL: "", Model: "m"},
	}
	for i, c := range cases {
		if err := svc.TestInput(ctx, c); !errors.Is(err, aiconfig.ErrInvalid()) {
			t.Errorf("case %d: want ErrInvalid, got %v", i, err)
		}
	}

	if err := svc.TestInput(ctx, aiconfig.Input{
		Kind: models.AIProviderOpenAICompat, BaseURL: "http://localhost:11434/v1", Model: "llama3",
	}); err != nil {
		t.Fatalf("compat draft without key should be testable: %v", err)
	}
}

func TestValidationRejectsBadInput(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()

	cases := []aiconfig.Input{
		{Kind: "bogus", Name: "x", APIKey: "k", Model: "m"},
		{Kind: models.AIProviderOpenAI, Name: "x", APIKey: "", Model: "m"},
		{Kind: models.AIProviderOpenAICompat, Name: "x", Model: "m"}, // no base URL
		{Kind: models.AIProviderOpenAICompat, BaseURL: "http://localhost:11434/v1", Model: "m"},
		{Kind: models.AIProviderOpenAI, Name: "x", APIKey: "k", Model: ""},
	}
	for i, c := range cases {
		if _, err := svc.Create(ctx, "user-1", c); !errors.Is(err, aiconfig.ErrInvalid()) {
			t.Errorf("case %d: want ErrInvalid, got %v", i, err)
		}
	}

	// openai_compatible with base URL but no key is allowed (local endpoints).
	if _, err := svc.Create(ctx, "user-1", aiconfig.Input{
		Kind: models.AIProviderOpenAICompat, Name: "Ollama", BaseURL: "http://localhost:11434/v1", Model: "llama3",
	}); err != nil {
		t.Fatalf("compat without key should be allowed: %v", err)
	}
}

func TestBuiltinProviderNameDefaults(t *testing.T) {
	svc, _ := newService(t, config.AIConfig{})
	ctx := context.Background()

	sum, err := svc.Create(ctx, "user-1", aiconfig.Input{
		Kind:   models.AIProviderOpenRouter,
		APIKey: "sk-router",
		Model:  "openai/gpt-4o",
	})
	if err != nil {
		t.Fatalf("create openrouter without name: %v", err)
	}
	if sum.Name != "OpenRouter" {
		t.Fatalf("name = %q, want OpenRouter", sum.Name)
	}
}

func TestGlobalStatusProjection(t *testing.T) {
	off, _ := newService(t, config.AIConfig{})
	if off.Global().Configured {
		t.Fatal("empty global should be not-configured")
	}

	on, _ := newService(t, config.AIConfig{
		Kind: "openai", Name: "Shared", APIKey: "sk-x", Model: "gpt-4o",
	})
	g := on.Global()
	if !g.Configured || g.Provider != "Shared" || g.Model != "gpt-4o" {
		t.Fatalf("unexpected global status %+v", g)
	}

	bad, _ := newService(t, config.AIConfig{
		Kind: "AI", Name: "Shared", APIKey: "sk-x", Model: "gpt-4o",
	})
	if got := bad.Global(); !got.Configured || got.Usable || got.Kind != "AI" {
		t.Fatalf("unsupported global kind should stay visible but unusable: %+v", got)
	}

	upper, _ := newService(t, config.AIConfig{
		Kind: "OpenAI", Name: "Shared", APIKey: "sk-x", Model: "gpt-4o",
	})
	if got := upper.Global(); !got.Configured || !got.Usable || got.Kind != "openai" {
		t.Fatalf("uppercase global kind was not normalized: %+v", got)
	}
}
