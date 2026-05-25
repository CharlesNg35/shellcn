package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/store"
)

// CredentialService owns reusable credentials: it encrypts secret material on
// write, resolves it for connect-time injection (authorized callers only), and
// lists the non-secret summaries a user may select from. Secret values never
// leave this layer toward the client.
type CredentialService struct {
	creds          store.CredentialStore
	grants         store.CredentialGrantStore
	vault          secrets.SecretStore
	onSecretAccess func()
}

// NewCredentialService wires the dependencies.
func NewCredentialService(creds store.CredentialStore, grants store.CredentialGrantStore, vault secrets.SecretStore) *CredentialService {
	return &CredentialService{creds: creds, grants: grants, vault: vault}
}

// SetSecretAccessHook registers a callback for successful secret decryptions.
func (s *CredentialService) SetSecretAccessHook(fn func()) {
	s.onSecretAccess = fn
}

// NewCredentialInput describes a credential to create (secret in plaintext;
// encrypted before it touches the store).
type NewCredentialInput struct {
	OwnerID   string
	Name      string
	Kind      string
	Identity  string
	Protocols []string
	Secret    string
}

// Create encrypts the secret material and persists the credential.
func (s *CredentialService) Create(ctx context.Context, in NewCredentialInput) (models.Credential, error) {
	normalized, err := normalizeCredentialInput(in.Name, in.Kind, in.Identity, in.Protocols, true, in.Secret)
	if err != nil {
		return models.Credential{}, err
	}
	enc, err := s.vault.Encrypt(ctx, []byte(in.Secret))
	if err != nil {
		return models.Credential{}, fmt.Errorf("encrypt credential: %w", err)
	}
	now := time.Now()
	cred := models.Credential{
		ID:              uuid.NewString(),
		Name:            normalized.name,
		Kind:            normalized.kind,
		OwnerID:         in.OwnerID,
		Username:        normalized.identity,
		Protocols:       normalized.protocols,
		EncryptedSecret: enc,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.creds.Create(ctx, &cred); err != nil {
		return models.Credential{}, err
	}
	return cred, nil
}

// UpdateCredentialInput updates a credential's metadata and optionally rotates
// its secret. A blank Secret keeps the stored material (write-only).
type UpdateCredentialInput struct {
	Name      string
	Kind      string
	Identity  string
	Protocols []string
	Secret    string
}

// Update applies metadata changes and, when Secret is non-blank, rotates the
// encrypted material. Rotation updates the single record; every connection that
// references it resolves the new value on its next connect.
func (s *CredentialService) Update(ctx context.Context, id string, in UpdateCredentialInput) (models.Credential, error) {
	cred, err := s.creds.Get(ctx, id)
	if err != nil {
		return models.Credential{}, err
	}
	normalized, err := normalizeCredentialInput(in.Name, in.Kind, in.Identity, in.Protocols, false, in.Secret)
	if err != nil {
		return models.Credential{}, err
	}
	cred.Name = normalized.name
	cred.Kind = normalized.kind
	cred.Username = normalized.identity
	cred.Protocols = normalized.protocols
	if strings.TrimSpace(in.Secret) != "" {
		enc, err := s.vault.Encrypt(ctx, []byte(in.Secret))
		if err != nil {
			return models.Credential{}, fmt.Errorf("encrypt credential: %w", err)
		}
		cred.EncryptedSecret = enc
	}
	cred.UpdatedAt = time.Now()
	if err := s.creds.Update(ctx, &cred); err != nil {
		return models.Credential{}, err
	}
	return cred, nil
}

type normalizedCredentialInput struct {
	name      string
	kind      string
	identity  string
	protocols []string
}

func normalizeCredentialInput(name, kind, identity string, protocols []string, requireSecret bool, secret string) (normalizedCredentialInput, error) {
	out := normalizedCredentialInput{
		name:     strings.TrimSpace(name),
		kind:     strings.TrimSpace(kind),
		identity: strings.TrimSpace(identity),
	}
	if out.name == "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: credential name is required", plugin.ErrInvalidInput)
	}
	if out.kind == "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: credential kind is required", plugin.ErrInvalidInput)
	}
	info, ok := plugin.CredentialKindLookup(plugin.CredentialKind(out.kind))
	if !ok {
		return normalizedCredentialInput{}, fmt.Errorf("%w: unknown credential kind %q", plugin.ErrInvalidInput, out.kind)
	}
	if requireSecret && strings.TrimSpace(secret) == "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: secret material is required", plugin.ErrInvalidInput)
	}
	if info.IdentityLabel == "" && out.identity != "" {
		return normalizedCredentialInput{}, fmt.Errorf("%w: credential kind %q does not use identity metadata", plugin.ErrInvalidInput, out.kind)
	}
	seen := map[string]bool{}
	for _, protocol := range protocols {
		protocol = strings.TrimSpace(protocol)
		if protocol == "" || seen[protocol] {
			continue
		}
		if !plugin.CredentialKindSupportsProtocol(plugin.CredentialKind(out.kind), protocol) {
			return normalizedCredentialInput{}, fmt.Errorf("%w: credential kind %q is not compatible with protocol %q", plugin.ErrInvalidInput, out.kind, protocol)
		}
		seen[protocol] = true
		out.protocols = append(out.protocols, protocol)
	}
	return out, nil
}

// Delete removes a credential. Callers must enforce the not-referenced
// invariant — a credential still referenced by a connection cannot be deleted.
func (s *CredentialService) Delete(ctx context.Context, id string) error {
	return s.creds.Delete(ctx, id)
}

// canUse reports whether userID owns the credential or holds a use-grant.
func (s *CredentialService) canUse(ctx context.Context, userID string, cred models.Credential) (bool, error) {
	if cred.OwnerID == userID {
		return true, nil
	}
	return s.grants.Has(ctx, cred.ID, userID)
}

// EnsureUsable verifies that userID may use credentialID — used by the
// connection control plane when a config references a reusable credential. It
// returns ErrForbidden when the user lacks owner/use access and the store's
// not-found error for an unknown credential.
func (s *CredentialService) EnsureUsable(ctx context.Context, userID, credentialID string) error {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return err
	}
	return s.ensureUsableCredential(ctx, userID, cred)
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

// Resolve returns the decrypted secret material for connect-time injection IF
// the user may use the credential. The value is for the plugin's ConnectConfig
// only and must never be serialized back to the client.
func (s *CredentialService) Resolve(ctx context.Context, userID, credentialID string) ([]byte, error) {
	cred, err := s.creds.Get(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	ok, err := s.canUse(ctx, userID, cred)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("credential %q: %w", credentialID, models.ErrForbidden)
	}
	secret, err := s.vault.Decrypt(ctx, cred.EncryptedSecret)
	if err != nil {
		return nil, err
	}
	if s.onSecretAccess != nil {
		s.onSecretAccess()
	}
	return secret, nil
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
			if !plugin.CredentialKindSupportsProtocol(plugin.CredentialKind(cred.Kind), protocol) {
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
