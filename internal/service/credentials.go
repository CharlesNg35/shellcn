package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// CredentialService owns reusable credential encryption, resolution, and
// non-secret summaries.
type CredentialService struct {
	creds          store.CredentialStore
	grants         store.CredentialGrantStore
	vault          secrets.SecretStore
	kinds          plugin.CredentialKindCatalog
	onSecretAccess func()
}

type CredentialServiceOption func(*CredentialService)

// WithCredentialKindCatalog makes credential validation use the effective
// plugin registry catalog instead of only the core built-in kinds.
func WithCredentialKindCatalog(kinds plugin.CredentialKindCatalog) CredentialServiceOption {
	return func(s *CredentialService) {
		if kinds != nil {
			s.kinds = kinds
		}
	}
}

func NewCredentialService(creds store.CredentialStore, grants store.CredentialGrantStore, vault secrets.SecretStore, opts ...CredentialServiceOption) *CredentialService {
	svc := &CredentialService{
		creds:  creds,
		grants: grants,
		vault:  vault,
		kinds:  plugin.MustCredentialKindSet(plugin.BuiltInCredentialKinds()),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// SetSecretAccessHook registers a callback for successful secret decryptions.
func (s *CredentialService) SetSecretAccessHook(fn func()) {
	s.onSecretAccess = fn
}

// NewCredentialInput describes a credential to create.
type NewCredentialInput struct {
	OwnerID string
	Name    string
	Kind    string
	Values  map[string]string
}

// Create encrypts the secret material and persists the credential.
func (s *CredentialService) Create(ctx context.Context, in NewCredentialInput) (models.Credential, error) {
	normalized, err := s.normalizeCredentialInput(in.Name, in.Kind, in.Values, nil)
	if err != nil {
		return models.Credential{}, err
	}
	enc, err := s.encryptSecretValues(ctx, normalized.secretValues)
	if err != nil {
		return models.Credential{}, err
	}
	now := time.Now()
	cred := models.Credential{
		ID:              uuid.NewString(),
		Name:            normalized.name,
		Kind:            normalized.kind,
		OwnerID:         in.OwnerID,
		Values:          normalized.publicValues,
		Protocols:       normalized.protocols,
		EncryptedValues: enc,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.creds.Create(ctx, &cred); err != nil {
		return models.Credential{}, err
	}
	return cred, nil
}

// UpdateCredentialInput updates metadata and optionally rotates the secret.
type UpdateCredentialInput struct {
	Name   string
	Kind   string
	Values map[string]string
}

// Update applies metadata changes and rotates the encrypted material when set.
func (s *CredentialService) Update(ctx context.Context, id string, in UpdateCredentialInput) (models.Credential, error) {
	cred, err := s.creds.Get(ctx, id)
	if err != nil {
		return models.Credential{}, err
	}
	existingSecrets, err := s.decryptSecretValues(ctx, cred.EncryptedValues)
	if err != nil {
		return models.Credential{}, err
	}
	normalized, err := s.normalizeCredentialInput(in.Name, in.Kind, in.Values, existingSecrets)
	if err != nil {
		return models.Credential{}, err
	}
	cred.Name = normalized.name
	cred.Kind = normalized.kind
	cred.Values = normalized.publicValues
	cred.Protocols = normalized.protocols
	enc, err := s.encryptSecretValues(ctx, normalized.secretValues)
	if err != nil {
		return models.Credential{}, err
	}
	cred.EncryptedValues = enc
	cred.UpdatedAt = time.Now()
	if err := s.creds.Update(ctx, &cred); err != nil {
		return models.Credential{}, err
	}
	return cred, nil
}

type normalizedCredentialInput struct {
	name         string
	kind         string
	publicValues map[string]string
	secretValues map[string]string
	protocols    []string
}

func (s *CredentialService) normalizeCredentialInput(name, kind string, values map[string]string, existingSecrets map[string]string) (normalizedCredentialInput, error) {
	out := normalizedCredentialInput{
		name:         strings.TrimSpace(name),
		kind:         strings.TrimSpace(kind),
		publicValues: map[string]string{},
		secretValues: map[string]string{},
	}
	if out.name == "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: credential name is required", plugin.ErrInvalidInput)
	}
	if out.kind == "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: credential kind is required", plugin.ErrInvalidInput)
	}
	info, ok := s.kinds.CredentialKindLookup(plugin.CredentialKind(out.kind))
	if !ok {
		return normalizedCredentialInput{}, fmt.Errorf("%w: unknown credential kind %q", plugin.ErrInvalidInput, out.kind)
	}
	fields := map[string]plugin.Field{}
	for _, field := range info.Fields {
		fields[field.Key] = field
	}
	for key, value := range values {
		if _, ok := fields[key]; !ok && strings.TrimSpace(value) != "" {
			return normalizedCredentialInput{}, fmt.Errorf("%w: credential kind %q does not declare field %q", plugin.ErrInvalidInput, out.kind, key)
		}
	}
	for _, field := range info.Fields {
		value := strings.TrimSpace(values[field.Key])
		if field.Secret {
			if value == "" && existingSecrets != nil {
				value = existingSecrets[field.Key]
			}
			if field.Required && value == "" {
				return normalizedCredentialInput{}, fmt.Errorf("%w: credential field %q is required", plugin.ErrInvalidInput, field.Key)
			}
			if value != "" {
				out.secretValues[field.Key] = value
			}
			continue
		}
		if field.Required && value == "" {
			return normalizedCredentialInput{}, fmt.Errorf("%w: credential field %q is required", plugin.ErrInvalidInput, field.Key)
		}
		if value != "" && field.Public {
			out.publicValues[field.Key] = value
		}
	}
	out.protocols = append(out.protocols, info.CompatibleProtocols...)
	return out, nil
}

func (s *CredentialService) encryptSecretValues(ctx context.Context, values map[string]string) ([]byte, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode credential values: %w", err)
	}
	enc, err := s.vault.Encrypt(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("encrypt credential values: %w", err)
	}
	return enc, nil
}

func (s *CredentialService) decryptSecretValues(ctx context.Context, enc []byte) (map[string]string, error) {
	if len(enc) == 0 {
		return map[string]string{}, nil
	}
	raw, err := s.vault.Decrypt(ctx, enc)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode credential values: %w", err)
	}
	return values, nil
}

// Delete removes a credential after callers enforce reference checks.
func (s *CredentialService) Delete(ctx context.Context, id string) error {
	return s.creds.Delete(ctx, id)
}

// canUse reports whether userID owns the credential or holds a view-grant.
func (s *CredentialService) canUse(ctx context.Context, userID string, cred models.Credential) (bool, error) {
	if cred.OwnerID == userID {
		return true, nil
	}
	return s.grants.Has(ctx, cred.ID, userID)
}

// EnsureUsable verifies owner/use access to a credential.
func (s *CredentialService) EnsureUsable(ctx context.Context, userID, credentialID string) error {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return err
	}
	return s.ensureUsableCredential(ctx, userID, cred)
}

// SummaryIfUsable returns a non-secret summary only when userID has use access.
func (s *CredentialService) SummaryIfUsable(ctx context.Context, userID, credentialID string) (models.CredentialSummary, bool) {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return models.CredentialSummary{}, false
	}
	ok, err := s.canUse(ctx, userID, cred)
	if err != nil || !ok {
		return models.CredentialSummary{}, false
	}
	summary := cred.Summary()
	return summary, true
}

// EnsureUsableFor verifies that userID may use credentialID and that the
// credential matches the selector constraints for the connection protocol.
func (s *CredentialService) EnsureUsableFor(ctx context.Context, userID, credentialID string, kinds []string, protocol string) error {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return err
	}
	if len(kinds) > 0 && !slices.Contains(kinds, cred.Kind) {
		return fmt.Errorf("%w: credential %q is kind %q", plugin.ErrInvalidInput, credentialID, cred.Kind)
	}
	if protocol != "" && !s.kinds.CredentialKindSupportsProtocol(plugin.CredentialKind(cred.Kind), protocol) {
		return fmt.Errorf("%w: credential kind %q is not compatible with protocol %q", plugin.ErrInvalidInput, cred.Kind, protocol)
	}
	if protocol != "" && len(cred.Protocols) > 0 && !slices.Contains(cred.Protocols, protocol) {
		return fmt.Errorf("%w: credential %q is not valid for protocol %q", plugin.ErrInvalidInput, credentialID, protocol)
	}
	return s.ensureUsableCredential(ctx, userID, cred)
}

func (s *CredentialService) ensureUsableCredential(ctx context.Context, userID string, cred models.Credential) error {
	ok, err := s.canUse(ctx, userID, cred)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("credential %q: %w", cred.ID, models.ErrForbidden)
	}
	return nil
}

// ResolveWithMetadata returns metadata plus decrypted material after use access.
func (s *CredentialService) ResolveWithMetadata(ctx context.Context, userID, credentialID string) (models.Credential, map[string]string, error) {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return models.Credential{}, nil, err
	}
	ok, err := s.canUse(ctx, userID, cred)
	if err != nil {
		return models.Credential{}, nil, err
	}
	if !ok {
		return models.Credential{}, nil, fmt.Errorf("credential %q: %w", credentialID, models.ErrForbidden)
	}
	secrets, err := s.decryptSecretValues(ctx, cred.EncryptedValues)
	if err != nil {
		return models.Credential{}, nil, err
	}
	values := make(map[string]string, len(cred.Values)+len(secrets))
	for k, v := range cred.Values {
		values[k] = v
	}
	for k, v := range secrets {
		values[k] = v
	}
	if s.onSecretAccess != nil {
		s.onSecretAccess()
	}
	return cred, values, nil
}

// ListUsable returns the non-secret summaries the user may select for a
// credential_ref field, filtered by accepted kinds and an optional protocol.
func (s *CredentialService) ListUsable(ctx context.Context, userID string, kinds []string, protocol string) ([]models.CredentialSummary, error) {
	seen := map[string]bool{}
	var out []models.CredentialSummary

	consider := func(cred models.Credential) {
		if seen[cred.ID] {
			return
		}
		seen[cred.ID] = true
		if len(kinds) > 0 && !slices.Contains(kinds, cred.Kind) {
			return
		}
		if protocol != "" {
			if !s.kinds.CredentialKindSupportsProtocol(plugin.CredentialKind(cred.Kind), protocol) {
				return
			}
			// Empty Protocols means the credential works with any compatible protocol.
			if len(cred.Protocols) > 0 && !slices.Contains(cred.Protocols, protocol) {
				return
			}
		}
		out = append(out, cred.Summary())
	}

	owned, err := s.creds.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range owned {
		consider(c)
	}

	granted, err := s.grants.ListBySubject(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, g := range granted {
		cred, err := s.creds.Get(ctx, g.CredentialID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		consider(cred)
	}
	return out, nil
}
