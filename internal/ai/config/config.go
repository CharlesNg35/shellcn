// Package aiconfig manages AI provider configuration: a read-only global provider
// from internal/config, and owner-scoped per-user providers whose API keys are
// Vault-encrypted before the store and never returned to clients.
package aiconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
)

var errInvalid = errors.New("invalid ai provider config")

// ErrInvalid is the validation sentinel callers can match.
func ErrInvalid() error { return errInvalid }

// builtinKinds is the closed vocabulary validated at registration. Custom
// providers are openai_compatible rows with their own name + base URL.
var builtinKinds = map[models.AIProviderKind]bool{
	models.AIProviderOpenAI:       true,
	models.AIProviderOpenRouter:   true,
	models.AIProviderAnthropic:    true,
	models.AIProviderGoogle:       true,
	models.AIProviderOpenAICompat: true,
}

// defaultModels is the static fallback for the picker when no live catalogue or
// configured allow-list is available.
var defaultModels = map[models.AIProviderKind][]string{
	models.AIProviderOpenAI:     {"gpt-4o", "gpt-4o-mini", "o3-mini"},
	models.AIProviderOpenRouter: {"openai/gpt-4o", "anthropic/claude-sonnet-4.5", "google/gemini-2.5-pro"},
	models.AIProviderAnthropic:  {"claude-opus-4-1", "claude-sonnet-4-5", "claude-haiku-4-5"},
	models.AIProviderGoogle:     {"gemini-2.5-pro", "gemini-2.5-flash"},
}

// Input is a create/update request for a user provider. On update an empty
// APIKey keeps the stored ciphertext (write-only key semantics).
type Input struct {
	Kind         models.AIProviderKind
	Name         string
	BaseURL      string
	APIKey       string
	Models       []string
	DefaultModel string
}

// GlobalStatus is the read-only projection of the shared AI config exposed to
// clients: presence + provider/model, never the key.
type GlobalStatus struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Model      string `json:"model,omitempty"`
}

// Service is the user-provider CRUD + global-status surface.
type Service struct {
	store  store.AIProviderStore
	vault  secrets.SecretStore
	global config.AIConfig
	models *modelreg.Registry
}

// New wires the provider store, the secret vault, and the global config.
func New(s store.AIProviderStore, vault secrets.SecretStore, global config.AIConfig) *Service {
	return &Service{store: s, vault: vault, global: global}
}

// WithModels enables live model listing + provider connectivity tests.
func (s *Service) WithModels(r *modelreg.Registry) *Service {
	s.models = r
	return s
}

// Global returns the read-only shared-AI status (never the key).
func (s *Service) Global() GlobalStatus {
	if !s.global.Configured() {
		return GlobalStatus{Configured: false}
	}
	return GlobalStatus{
		Configured: true,
		Provider:   s.global.DisplayName(),
		Kind:       s.global.Kind,
		Model:      s.global.Model,
	}
}

// List returns the owner's providers as non-secret summaries.
func (s *Service) List(ctx context.Context, ownerID string) ([]models.AIProviderSummary, error) {
	rows, err := s.store.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]models.AIProviderSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Summary())
	}
	return out, nil
}

// Create validates input, encrypts the key, and persists a new owned provider.
func (s *Service) Create(ctx context.Context, ownerID string, in Input) (models.AIProviderSummary, error) {
	norm, err := s.normalize(in, true)
	if err != nil {
		return models.AIProviderSummary{}, err
	}
	now := time.Now()
	row := models.AIProviderConfig{
		ID:           uuid.NewString(),
		OwnerID:      ownerID,
		Kind:         norm.Kind,
		Name:         norm.Name,
		BaseURL:      norm.BaseURL,
		Models:       norm.Models,
		DefaultModel: norm.DefaultModel,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if norm.APIKey != "" {
		enc, err := s.vault.Encrypt(ctx, []byte(norm.APIKey))
		if err != nil {
			return models.AIProviderSummary{}, fmt.Errorf("encrypt api key: %w", err)
		}
		row.APIKeyCiphertext = enc
	}
	if err := s.store.Create(ctx, &row); err != nil {
		return models.AIProviderSummary{}, err
	}
	return row.Summary(), nil
}

// Update mutates an owned provider. An empty APIKey preserves the stored key.
func (s *Service) Update(ctx context.Context, ownerID, id string, in Input) (models.AIProviderSummary, error) {
	row, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return models.AIProviderSummary{}, err
	}
	norm, err := s.normalize(in, false)
	if err != nil {
		return models.AIProviderSummary{}, err
	}
	row.Kind = norm.Kind
	row.Name = norm.Name
	row.BaseURL = norm.BaseURL
	row.Models = norm.Models
	row.DefaultModel = norm.DefaultModel
	row.UpdatedAt = time.Now()
	if norm.APIKey != "" {
		enc, err := s.vault.Encrypt(ctx, []byte(norm.APIKey))
		if err != nil {
			return models.AIProviderSummary{}, fmt.Errorf("encrypt api key: %w", err)
		}
		row.APIKeyCiphertext = enc
	}
	if err := s.store.Update(ctx, &row); err != nil {
		return models.AIProviderSummary{}, err
	}
	return row.Summary(), nil
}

// Delete removes an owned provider.
func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return err
	}
	return s.store.Delete(ctx, id)
}

// Models lists a provider's selectable models. It queries the provider's live
// catalogue when possible, falling back to the configured allow-list, then a
// static per-kind default.
func (s *Service) Models(ctx context.Context, ownerID, id string) ([]string, error) {
	row, key, err := s.Resolve(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if s.models != nil {
		if live, err := s.models.FetchModels(ctx, string(row.Kind), row.BaseURL, key); err == nil && len(live) > 0 {
			ids := make([]string, 0, len(live))
			for _, m := range live {
				ids = append(ids, m.ID)
			}
			return ids, nil
		}
	}
	if len(row.Models) > 0 {
		return row.Models, nil
	}
	if m, ok := defaultModels[row.Kind]; ok {
		return m, nil
	}
	if row.DefaultModel != "" {
		return []string{row.DefaultModel}, nil
	}
	return []string{}, nil
}

// ModelsForInput lists models for an unsaved provider draft. It is used by the
// settings UI to fetch a provider catalog before persisting the key.
func (s *Service) ModelsForInput(ctx context.Context, in Input) ([]string, error) {
	in.Kind = models.AIProviderKind(strings.TrimSpace(string(in.Kind)))
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)
	if !builtinKinds[in.Kind] {
		return nil, fmt.Errorf("%w: unknown kind %q", errInvalid, in.Kind)
	}
	if in.Kind == models.AIProviderOpenAICompat && in.BaseURL == "" {
		return nil, fmt.Errorf("%w: base URL is required for an openai-compatible provider", errInvalid)
	}
	if s.models != nil {
		if live, err := s.models.FetchModels(ctx, string(in.Kind), in.BaseURL, in.APIKey); err == nil && len(live) > 0 {
			ids := make([]string, 0, len(live))
			for _, m := range live {
				ids = append(ids, m.ID)
			}
			return ids, nil
		}
	}
	if len(in.Models) > 0 {
		return in.Models, nil
	}
	if m, ok := defaultModels[in.Kind]; ok {
		return m, nil
	}
	if in.DefaultModel != "" {
		return []string{in.DefaultModel}, nil
	}
	return []string{}, nil
}

// Test verifies a provider's credentials and endpoint by listing its models. It
// returns nil when the provider is reachable and authorized.
func (s *Service) Test(ctx context.Context, ownerID, id string) error {
	row, key, err := s.Resolve(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if s.models == nil {
		return nil
	}
	_, err = s.models.FetchModels(ctx, string(row.Kind), row.BaseURL, key)
	return err
}

// Resolve returns an owned provider with its decrypted API key for use at chat
// time. The plaintext key never leaves the AI service.
func (s *Service) Resolve(ctx context.Context, ownerID, id string) (models.AIProviderConfig, string, error) {
	row, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return models.AIProviderConfig{}, "", err
	}
	if len(row.APIKeyCiphertext) == 0 {
		return row, "", nil
	}
	key, err := s.vault.Decrypt(ctx, row.APIKeyCiphertext)
	if err != nil {
		return models.AIProviderConfig{}, "", err
	}
	return row, string(key), nil
}

// owned fetches a provider and enforces ownership, hiding others' rows as 404.
func (s *Service) owned(ctx context.Context, ownerID, id string) (models.AIProviderConfig, error) {
	row, err := s.store.Get(ctx, id)
	if err != nil {
		return models.AIProviderConfig{}, err
	}
	if row.OwnerID != ownerID {
		return models.AIProviderConfig{}, store.ErrNotFound
	}
	return row, nil
}

func (s *Service) normalize(in Input, create bool) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.DefaultModel = strings.TrimSpace(in.DefaultModel)

	if !builtinKinds[in.Kind] {
		return Input{}, fmt.Errorf("%w: unknown kind %q", errInvalid, in.Kind)
	}
	if in.Name == "" && in.Kind != models.AIProviderOpenAICompat {
		in.Name = defaultProviderName(in.Kind)
	}
	if in.Name == "" {
		return Input{}, fmt.Errorf("%w: name is required", errInvalid)
	}
	if in.Kind == models.AIProviderOpenAICompat && in.BaseURL == "" {
		return Input{}, fmt.Errorf("%w: base URL is required for an openai-compatible provider", errInvalid)
	}
	if in.DefaultModel == "" {
		return Input{}, fmt.Errorf("%w: default model is required", errInvalid)
	}
	// Named vendors require a key; openai_compatible (e.g. local Ollama) may not.
	if create && in.APIKey == "" && in.Kind != models.AIProviderOpenAICompat {
		return Input{}, fmt.Errorf("%w: api key is required", errInvalid)
	}
	cleaned := make([]string, 0, len(in.Models))
	for _, m := range in.Models {
		if m = strings.TrimSpace(m); m != "" {
			cleaned = append(cleaned, m)
		}
	}
	in.Models = cleaned
	return in, nil
}

func defaultProviderName(kind models.AIProviderKind) string {
	switch kind {
	case models.AIProviderOpenAI:
		return "OpenAI"
	case models.AIProviderOpenRouter:
		return "OpenRouter"
	case models.AIProviderAnthropic:
		return "Anthropic"
	case models.AIProviderGoogle:
		return "Google"
	default:
		return ""
	}
}
