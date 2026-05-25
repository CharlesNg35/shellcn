package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/store"
)

// CredentialService owns reusable credentials: it encrypts secret material on
// write, resolves it for connect-time injection (authorized callers only), and
// lists the non-secret summaries a user may select from. Secret values never
// leave this layer toward the client.
type CredentialService struct {
	creds  store.CredentialStore
	grants store.CredentialGrantStore
	vault  secrets.SecretStore
}

// NewCredentialService wires the dependencies.
func NewCredentialService(creds store.CredentialStore, grants store.CredentialGrantStore, vault secrets.SecretStore) *CredentialService {
	return &CredentialService{creds: creds, grants: grants, vault: vault}
}

// NewCredentialInput describes a credential to create (secret in plaintext;
// encrypted before it touches the store).
type NewCredentialInput struct {
	OwnerID   string
	Name      string
	Kind      string
	Username  string
	Protocols []string
	Secret    string
}

// Create encrypts the secret material and persists the credential.
func (s *CredentialService) Create(ctx context.Context, in NewCredentialInput) (models.Credential, error) {
	enc, err := s.vault.Encrypt(ctx, []byte(in.Secret))
	if err != nil {
		return models.Credential{}, fmt.Errorf("encrypt credential: %w", err)
	}
	now := time.Now()
	cred := models.Credential{
		ID:              uuid.NewString(),
		Name:            in.Name,
		Kind:            in.Kind,
		OwnerID:         in.OwnerID,
		Username:        in.Username,
		Protocols:       in.Protocols,
		EncryptedSecret: enc,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.creds.Create(ctx, &cred); err != nil {
		return models.Credential{}, err
	}
	return cred, nil
}

// canUse reports whether userID owns the credential or holds a use-grant.
func (s *CredentialService) canUse(ctx context.Context, userID string, cred models.Credential) (bool, error) {
	if cred.OwnerID == userID {
		return true, nil
	}
	return s.grants.Has(ctx, cred.ID, userID)
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
	return s.vault.Decrypt(ctx, cred.EncryptedSecret)
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
		// Empty Protocols means the credential works with any compatible protocol.
		if protocol != "" && len(cred.Protocols) > 0 && !slices.Contains(cred.Protocols, protocol) {
			return
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
