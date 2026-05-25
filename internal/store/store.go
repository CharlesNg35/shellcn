// Package store is the cross-database control-plane store. GORM lives here and
// nowhere else; the rest of the app depends only on the repository interfaces.
package store

import (
	"context"
	"errors"

	"github.com/charlesng/shellcn/internal/models"
)

// ErrNotFound is returned when a record does not exist. Backends normalize their
// own not-found errors to this sentinel.
var ErrNotFound = errors.New("record not found")

// UserStore persists platform users and their local password hashes.
type UserStore interface {
	Create(ctx context.Context, u *models.User, passwordHash string) error
	GetByID(ctx context.Context, id string) (models.User, error)
	GetByUsername(ctx context.Context, username string) (models.User, error)
	GetPasswordHash(ctx context.Context, userID string) (string, error)
	SetPasswordHash(ctx context.Context, userID, hash string) error
	List(ctx context.Context) ([]models.User, error)
	Update(ctx context.Context, u *models.User) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

// ConnectionStore persists connections (with ciphertext for inline secrets).
type ConnectionStore interface {
	Create(ctx context.Context, c *models.Connection) error
	Get(ctx context.Context, id string) (models.Connection, error)
	ListByOwner(ctx context.Context, ownerID string) ([]models.Connection, error)
	Update(ctx context.Context, c *models.Connection) error
	Delete(ctx context.Context, id string) error
}

// CredentialStore persists reusable credentials (with ciphertext material).
type CredentialStore interface {
	Create(ctx context.Context, c *models.Credential) error
	Get(ctx context.Context, id string) (models.Credential, error)
	ListByOwner(ctx context.Context, ownerID string) ([]models.Credential, error)
	Update(ctx context.Context, c *models.Credential) error
	Delete(ctx context.Context, id string) error
}

// GrantStore persists per-connection sharing grants.
type GrantStore interface {
	Create(ctx context.Context, g *models.Grant) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, connectionID, subjectID string) (models.Grant, error)
	ListByConnection(ctx context.Context, connectionID string) ([]models.Grant, error)
	ListBySubject(ctx context.Context, subjectID string) ([]models.Grant, error)
}

// CredentialGrantStore persists credential use-grants (no secret readback).
type CredentialGrantStore interface {
	Create(ctx context.Context, g *models.CredentialGrant) error
	Delete(ctx context.Context, id string) error
	Has(ctx context.Context, credentialID, subjectID string) (bool, error)
	ListBySubject(ctx context.Context, subjectID string) ([]models.CredentialGrant, error)
}

// AuditStore is append-only: records are written and read, never updated/deleted.
type AuditStore interface {
	Append(ctx context.Context, e *models.AuditEntry) error
	List(ctx context.Context, f AuditFilter) ([]models.AuditEntry, error)
}

// AuditFilter narrows an audit query.
type AuditFilter struct {
	UserID       string
	ConnectionID string
	Limit        int
}

// SnippetStore persists saved command/query snippets.
type SnippetStore interface {
	Create(ctx context.Context, s *models.Snippet) error
	Get(ctx context.Context, id string) (models.Snippet, error)
	ListByOwner(ctx context.Context, ownerID, protocol string) ([]models.Snippet, error)
	Update(ctx context.Context, s *models.Snippet) error
	Delete(ctx context.Context, id string) error
}

// PreferenceStore persists per-user key/value preferences.
type PreferenceStore interface {
	Get(ctx context.Context, userID, key string) (models.Preference, error)
	Set(ctx context.Context, p *models.Preference) error
	Delete(ctx context.Context, userID, key string) error
}

// EnrollmentStore persists agent enrollment lifecycle records.
type EnrollmentStore interface {
	Create(ctx context.Context, e *models.AgentEnrollment) error
	Get(ctx context.Context, id string) (models.AgentEnrollment, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (models.AgentEnrollment, error)
	ListByConnection(ctx context.Context, connectionID string) ([]models.AgentEnrollment, error)
	UpdateStatus(ctx context.Context, id string, status models.AgentEnrollmentStatus) error
}

// PolicyStore persists additive authorization policies loaded into Casbin.
type PolicyStore interface {
	Create(ctx context.Context, p *models.PolicyRule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]models.PolicyRule, error)
}

// Store aggregates every repository plus lifecycle controls.
type Store struct {
	Users            UserStore
	Connections      ConnectionStore
	Credentials      CredentialStore
	Grants           GrantStore
	CredentialGrants CredentialGrantStore
	Audit            AuditStore
	Snippets         SnippetStore
	Preferences      PreferenceStore
	Enrollments      EnrollmentStore
	Policies         PolicyStore

	close func() error
}

// Close releases the underlying database, if any.
func (s *Store) Close() error {
	if s.close == nil {
		return nil
	}
	return s.close()
}
