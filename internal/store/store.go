// Package store is the cross-database control-plane store. GORM lives here and
// nowhere else; the rest of the app depends only on the repository interfaces.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
)

// ErrNotFound is returned when a record does not exist. Backends normalize their
// own not-found errors to this sentinel.
var ErrNotFound = errors.New("record not found")

// UserStore persists platform users and their local password hashes.
type UserStore interface {
	Create(ctx context.Context, u *models.User, passwordHash string) error
	GetByID(ctx context.Context, id string) (models.User, error)
	GetByUsername(ctx context.Context, username string) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	GetPasswordHash(ctx context.Context, userID string) (string, error)
	SetPasswordHash(ctx context.Context, userID, hash string) error
	List(ctx context.Context) ([]models.User, error)
	Update(ctx context.Context, u *models.User) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)

	// SetTwoFactor persists the user's TOTP enrollment state atomically: the
	// encrypted secret, the enabled flag, and the hashed recovery codes.
	SetTwoFactor(ctx context.Context, userID string, secret []byte, enabled bool, recoveryHashes []string) error
	// SetMFARemindedAt records when the user was last nudged to enable 2FA.
	SetMFARemindedAt(ctx context.Context, userID string, at *time.Time) error
}

// ConnectionStore persists connections (with ciphertext for inline secrets).
type ConnectionStore interface {
	Create(ctx context.Context, c *models.Connection) error
	Get(ctx context.Context, id string) (models.Connection, error)
	ListByOwner(ctx context.Context, ownerID string) ([]models.Connection, error)
	List(ctx context.Context) ([]models.Connection, error)
	Update(ctx context.Context, c *models.Connection) error
	Delete(ctx context.Context, id string) error
}

// ConnectionFolderStore persists per-user connection folders.
type ConnectionFolderStore interface {
	Create(ctx context.Context, f *models.ConnectionFolder) error
	Get(ctx context.Context, id string) (models.ConnectionFolder, error)
	ListByUser(ctx context.Context, userID string) ([]models.ConnectionFolder, error)
	Update(ctx context.Context, f *models.ConnectionFolder) error
	Delete(ctx context.Context, id string) error
}

// ConnectionPlacementStore persists per-user connection ordering and foldering.
type ConnectionPlacementStore interface {
	ListByUser(ctx context.Context, userID string) ([]models.ConnectionPlacement, error)
	Set(ctx context.Context, p *models.ConnectionPlacement) error
	Delete(ctx context.Context, userID, connectionID string) error
	DeleteByConnection(ctx context.Context, connectionID string) error
	ClearFolder(ctx context.Context, userID, folderID string) error
	MoveFolder(ctx context.Context, userID, folderID, targetFolderID string) error
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
	ListByCredential(ctx context.Context, credentialID string) ([]models.CredentialGrant, error)
	ListBySubject(ctx context.Context, subjectID string) ([]models.CredentialGrant, error)
}

// AuditStore is append-only: records are written and read, never updated/deleted.
type AuditStore interface {
	Append(ctx context.Context, e *models.AuditEntry) error
	List(ctx context.Context, f AuditFilter) ([]models.AuditEntry, error)
	// Count returns the number of entries matching the filter (Limit/Offset ignored).
	Count(ctx context.Context, f AuditFilter) (int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

// RecordingStore persists session-recording metadata (the blobs live elsewhere).
type RecordingStore interface {
	Create(ctx context.Context, r *models.Recording) error
	Get(ctx context.Context, id string) (models.Recording, error)
	Update(ctx context.Context, r *models.Recording) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f RecordingFilter) ([]models.Recording, error)
}

// RecordingFilter narrows a recording query. Zero-value fields are ignored.
type RecordingFilter struct {
	UserID       string
	ConnectionID string
	Protocol     string
	Class        string
	Format       string
	Status       string
	Since        time.Time
	Until        time.Time
	// ExpiredBefore selects recordings whose ExpiresAt is set and at/before it
	// (used by retention cleanup). Ignored when zero.
	ExpiredBefore time.Time
	Limit         int
}

// AuditFilter narrows an audit query.
type AuditFilter struct {
	UserID       string
	ConnectionID string
	Limit        int
	Offset       int
}

// PluginStorageFilter narrows generic plugin storage access. Collection,
// Plugin, and OwnerID are required for all operations. ConnectionID is optional
// for user-scoped reads/lists/deletes across the current user's connection rows.
type PluginStorageFilter struct {
	Collection    string
	Plugin        string
	ConnectionID  string
	OwnerID       string
	Key           string
	Keys          []string
	KeyPrefix     string
	ContentType   string
	CreatedAfter  time.Time
	CreatedBefore time.Time
	UpdatedAfter  time.Time
	UpdatedBefore time.Time
	Limit         int
	Offset        int
}

// PluginStorageStore persists scoped plugin-owned platform objects.
type PluginStorageStore interface {
	Get(ctx context.Context, f PluginStorageFilter) (models.PluginStorageItem, error)
	Put(ctx context.Context, item *models.PluginStorageItem) error
	Delete(ctx context.Context, f PluginStorageFilter) error
	List(ctx context.Context, f PluginStorageFilter) ([]models.PluginStorageItem, error)
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
	// UpdateToken sets the redeemable token hash and resets the install window.
	// Used by URL-delivered artifacts, which mint the real token only when the
	// artifact body is fetched (the record is created with a placeholder hash).
	UpdateToken(ctx context.Context, id, tokenHash string, expiresAt time.Time) error
	// Consume atomically transitions an install token to online. Pending tokens
	// must be unexpired; already enrolled offline/online tokens may reconnect.
	Consume(ctx context.Context, id string, now time.Time) (bool, error)
}

// PolicyStore persists additive authorization policies loaded into Casbin.
type PolicyStore interface {
	Create(ctx context.Context, p *models.PolicyRule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]models.PolicyRule, error)
}

// InvitationStore persists account invitations (only the token hash is stored).
type InvitationStore interface {
	Create(ctx context.Context, i *models.Invitation) error
	Get(ctx context.Context, id string) (models.Invitation, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (models.Invitation, error)
	List(ctx context.Context) ([]models.Invitation, error)
	Update(ctx context.Context, i *models.Invitation) error
	Consume(ctx context.Context, id string, acceptedAt time.Time) (bool, error)
	Delete(ctx context.Context, id string) error
}

// ProtocolSettingStore persists per-protocol availability states (admin-managed).
type ProtocolSettingStore interface {
	List(ctx context.Context) ([]models.ProtocolSetting, error)
	Set(ctx context.Context, s *models.ProtocolSetting) error
}

// AIProviderStore persists user-scoped AI provider configs (ciphertext keys).
type AIProviderStore interface {
	Create(ctx context.Context, c *models.AIProviderConfig) error
	Get(ctx context.Context, id string) (models.AIProviderConfig, error)
	ListByOwner(ctx context.Context, ownerID string) ([]models.AIProviderConfig, error)
	Update(ctx context.Context, c *models.AIProviderConfig) error
	Delete(ctx context.Context, id string) error
}

// AIConversationStore persists chat threads (user + connection scoped).
type AIConversationStore interface {
	Create(ctx context.Context, c *models.AIConversation) error
	Get(ctx context.Context, id string) (models.AIConversation, error)
	List(ctx context.Context, ownerID, connectionID string) ([]models.AIConversation, error)
	Update(ctx context.Context, c *models.AIConversation) error
	Delete(ctx context.Context, id string) error
}

// AIMessageStore persists conversation messages in sequence.
type AIMessageStore interface {
	Append(ctx context.Context, m *models.AIMessage) error
	List(ctx context.Context, conversationID string) ([]models.AIMessage, error)
	// Recent returns the newest limit messages, ordered oldest→newest.
	Recent(ctx context.Context, conversationID string, limit int) ([]models.AIMessage, error)
	// Range returns messages [offset, offset+limit) ordered oldest→newest.
	Range(ctx context.Context, conversationID string, offset, limit int) ([]models.AIMessage, error)
	Count(ctx context.Context, conversationID string) (int, error)
	DeleteByConversation(ctx context.Context, conversationID string) error
}

type LiveStateLeaseStore interface {
	Claim(ctx context.Context, lease *models.LiveStateLease, replace bool, now time.Time) (models.LiveStateLease, error)
	Get(ctx context.Context, key string, now time.Time) (models.LiveStateLease, error)
	Renew(ctx context.Context, key, leaseID string, expiresAt, now time.Time) (bool, error)
	PreferInternalURL(ctx context.Context, key, leaseID, internalURL string, now time.Time) (bool, error)
	Release(ctx context.Context, key, leaseID string) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

// Store aggregates every repository plus lifecycle controls.
type Store struct {
	Users                UserStore
	Connections          ConnectionStore
	ConnectionFolders    ConnectionFolderStore
	ConnectionPlacements ConnectionPlacementStore
	Credentials          CredentialStore
	Grants               GrantStore
	CredentialGrants     CredentialGrantStore
	Audit                AuditStore
	PluginStorage        PluginStorageStore
	Preferences          PreferenceStore
	Enrollments          EnrollmentStore
	Policies             PolicyStore
	Invitations          InvitationStore
	Recordings           RecordingStore
	ProtocolSettings     ProtocolSettingStore
	AIProviders          AIProviderStore
	AIConversations      AIConversationStore
	AIMessages           AIMessageStore
	LiveStateLeases      LiveStateLeaseStore

	close func() error
}

// Close releases the underlying database, if any.
func (s *Store) Close() error {
	if s.close == nil {
		return nil
	}
	return s.close()
}
