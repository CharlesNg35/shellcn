// Package aiconfig manages shared and user-scoped AI provider configuration.
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

// builtinKinds is the closed provider-kind vocabulary.
var builtinKinds = map[models.AIProviderKind]bool{
	models.AIProviderOpenAI:       true,
	models.AIProviderOpenRouter:   true,
	models.AIProviderAnthropic:    true,
	models.AIProviderGoogle:       true,
	models.AIProviderOpenAICompat: true,
}

// vendorModelCatalog is the static model picker fallback.
var vendorModelCatalog = map[models.AIProviderKind][]string{
	models.AIProviderOpenAI:     {"gpt-4o", "gpt-4o-mini", "o3-mini"},
	models.AIProviderOpenRouter: {"openai/gpt-4o", "anthropic/claude-sonnet-4.5", "google/gemini-2.5-pro"},
	models.AIProviderAnthropic:  {"claude-opus-4-1", "claude-sonnet-4-5", "claude-haiku-4-5"},
	models.AIProviderGoogle:     {"gemini-2.5-pro", "gemini-2.5-flash"},
}

type validationMode int

const (
	validateCreate validationMode = iota
	validateUpdate
	validateDraftTest
	validateCatalog
)

// Input is a create/update request for a user provider.
type Input struct {
	Kind    models.AIProviderKind
	Name    string
	BaseURL string
	APIKey  string
	Models  []string
	Model   string
}

// GlobalStatus is the read-only shared AI config projection.
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
	norm, err := s.validate(ctx, ownerID, "", in, validateCreate)
	if err != nil {
		return models.AIProviderSummary{}, err
	}
	now := time.Now()
	row := models.AIProviderConfig{
		ID:        uuid.NewString(),
		OwnerID:   ownerID,
		Kind:      norm.Kind,
		Name:      norm.Name,
		BaseURL:   norm.BaseURL,
		Models:    norm.Models,
		Model:     norm.Model,
		CreatedAt: now,
		UpdatedAt: now,
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
	norm, err := s.validate(ctx, ownerID, id, in, validateUpdate)
	if err != nil {
		return models.AIProviderSummary{}, err
	}
	if norm.APIKey == "" && requiresAPIKey(norm.Kind) && row.Kind != norm.Kind {
		return models.AIProviderSummary{}, fmt.Errorf("%w: api key is required", errInvalid)
	}
	row.Kind = norm.Kind
	row.Name = norm.Name
	row.BaseURL = norm.BaseURL
	row.Models = norm.Models
	row.Model = norm.Model
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

// ProviderModel returns the configured model for an owned provider without
// decrypting its API key.
func (s *Service) ProviderModel(ctx context.Context, ownerID, id string) (string, error) {
	row, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return "", err
	}
	return row.Model, nil
}

// Models lists a provider's selectable models.
func (s *Service) Models(ctx context.Context, ownerID, id string) ([]string, error) {
	row, key, err := s.Resolve(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if len(row.Models) > 0 {
		return row.Models, nil
	}
	if s.models != nil {
		live, err := s.models.FetchModels(ctx, string(row.Kind), row.BaseURL, key)
		if err != nil {
			return nil, err
		}
		return modelIDs(live), nil
	}
	if m, ok := vendorModelCatalog[row.Kind]; ok {
		return m, nil
	}
	if row.Model != "" {
		return []string{row.Model}, nil
	}
	return []string{}, nil
}

// ModelsForInput lists models for an unsaved provider draft.
func (s *Service) ModelsForInput(ctx context.Context, in Input) ([]string, error) {
	in, err := s.validate(ctx, "", "", in, validateCatalog)
	if err != nil {
		return nil, err
	}
	if s.models != nil {
		live, err := s.models.FetchModels(ctx, string(in.Kind), in.BaseURL, in.APIKey)
		if err != nil {
			return nil, err
		}
		return modelIDs(live), nil
	}
	if len(in.Models) > 0 {
		return in.Models, nil
	}
	if m, ok := vendorModelCatalog[in.Kind]; ok {
		return m, nil
	}
	if in.Model != "" {
		return []string{in.Model}, nil
	}
	return []string{}, nil
}

// TestInput verifies an unsaved provider draft without persisting the API key.
func (s *Service) TestInput(ctx context.Context, in Input) error {
	in, err := s.validate(ctx, "", "", in, validateDraftTest)
	if err != nil {
		return err
	}
	if s.models == nil {
		return nil
	}
	_, err = s.models.FetchModels(ctx, string(in.Kind), in.BaseURL, in.APIKey)
	return err
}

func modelIDs(models []modelreg.ProviderModel) []string {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		if strings.TrimSpace(m.ID) != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

// Test verifies a provider's credentials and endpoint by listing models.
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

func (s *Service) ensureUniqueName(ctx context.Context, ownerID, ignoreID, name string) error {
	rows, err := s.store.ListByOwner(ctx, ownerID)
	if err != nil {
		return err
	}
	want := strings.ToLower(name)
	for _, row := range rows {
		if row.ID != ignoreID && strings.ToLower(row.Name) == want {
			return fmt.Errorf("%w: provider name already exists", models.ErrConflict)
		}
	}
	return nil
}

func (s *Service) validate(ctx context.Context, ownerID, ignoreID string, in Input, mode validationMode) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.Model = strings.TrimSpace(in.Model)

	if !builtinKinds[in.Kind] {
		return Input{}, fmt.Errorf("%w: unknown kind %q", errInvalid, in.Kind)
	}
	if in.Kind == models.AIProviderOpenAICompat && in.BaseURL == "" {
		return Input{}, fmt.Errorf("%w: base URL is required for an openai-compatible provider", errInvalid)
	}
	if mode == validateCatalog {
		in.Models = cleanModels(in.Models)
		return in, nil
	}
	if in.Name == "" && in.Kind != models.AIProviderOpenAICompat {
		in.Name = defaultProviderName(in.Kind)
	}
	if mode != validateDraftTest && in.Name == "" {
		return Input{}, fmt.Errorf("%w: name is required", errInvalid)
	}
	if mode != validateCatalog && in.Model == "" {
		return Input{}, fmt.Errorf("%w: model is required", errInvalid)
	}
	if (mode == validateCreate || mode == validateDraftTest) && in.APIKey == "" && in.Kind != models.AIProviderOpenAICompat {
		return Input{}, fmt.Errorf("%w: api key is required", errInvalid)
	}
	in.Models = cleanModels(in.Models)
	if mode == validateCreate || mode == validateUpdate {
		if err := s.ensureUniqueName(ctx, ownerID, ignoreID, in.Name); err != nil {
			return Input{}, err
		}
	}
	return in, nil
}

func requiresAPIKey(kind models.AIProviderKind) bool {
	return kind != models.AIProviderOpenAICompat
}

func cleanModels(models []string) []string {
	cleaned := make([]string, 0, len(models))
	for _, m := range models {
		if m = strings.TrimSpace(m); m != "" {
			cleaned = append(cleaned, m)
		}
	}
	return cleaned
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
