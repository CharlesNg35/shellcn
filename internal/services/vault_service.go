package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/vault"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// ErrIdentityNotFound indicates the requested identity does not exist or is inaccessible.
var ErrIdentityNotFound = apperrors.ErrNotFound

// ErrShareNotFound indicates the requested share entry does not exist or is inaccessible.
var ErrShareNotFound = apperrors.ErrNotFound

// VaultService coordinates secure storage and retrieval of credential identities.
type VaultService struct {
	db      *gorm.DB
	audit   *AuditService
	checker PermissionChecker
	crypto  *vault.Crypto
}

// ViewerContext captures caller metadata used for permission enforcement.
type ViewerContext struct {
	UserID  string
	TeamIDs []string
	IsRoot  bool
}

// ListIdentitiesOptions controls filtering when listing identities.
type ListIdentitiesOptions struct {
	Scope                   models.IdentityScope
	ProtocolID              string
	IncludeConnectionScoped bool
}

// CreateIdentityInput defines the payload required to create a new identity.
type CreateIdentityInput struct {
	Name         string
	Description  string
	Scope        models.IdentityScope
	TemplateID   *string
	TeamID       *string
	ConnectionID *string
	Metadata     map[string]any
	Payload      map[string]any
	OwnerUserID  string
	CreatedBy    string
}

// UpdateIdentityInput defines mutable fields for an identity update.
type UpdateIdentityInput struct {
	Name         *string
	Description  *string
	Metadata     map[string]any
	Payload      map[string]any
	TemplateID   *string
	ConnectionID *string
	RotateClock  *time.Time
}

// IdentityShareInput defines the fields required to create an identity share.
type IdentityShareInput struct {
	PrincipalType models.IdentitySharePrincipal
	PrincipalID   string
	Permission    models.IdentitySharePermission
	ExpiresAt     *time.Time
	Metadata      map[string]any
	CreatedBy     string
}

// IdentityDTO represents an identity record returned to API consumers.
type IdentityDTO struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description,omitempty"`
	Scope           models.IdentityScope `json:"scope"`
	OwnerUserID     string               `json:"owner_user_id"`
	TeamID          *string              `json:"team_id,omitempty"`
	ConnectionID    *string              `json:"connection_id,omitempty"`
	TemplateID      *string              `json:"template_id,omitempty"`
	Version         int                  `json:"version"`
	Metadata        map[string]any       `json:"metadata,omitempty"`
	UsageCount      int                  `json:"usage_count"`
	LastUsedAt      *time.Time           `json:"last_used_at,omitempty"`
	LastRotatedAt   *time.Time           `json:"last_rotated_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	Payload         map[string]any       `json:"payload,omitempty"`
	Shares          []IdentityShareDTO   `json:"shares,omitempty"`
	ConnectionCount int                  `json:"connection_count"`
}

// IdentityShareDTO represents a share entry associated with an identity.
type IdentityShareDTO struct {
	ID            string                         `json:"id"`
	PrincipalType models.IdentitySharePrincipal  `json:"principal_type"`
	PrincipalID   string                         `json:"principal_id"`
	Permission    models.IdentitySharePermission `json:"permission"`
	ExpiresAt     *time.Time                     `json:"expires_at,omitempty"`
	Metadata      map[string]any                 `json:"metadata,omitempty"`
	GrantedBy     string                         `json:"granted_by"`
	CreatedBy     string                         `json:"created_by"`
	RevokedBy     *string                        `json:"revoked_by,omitempty"`
	RevokedAt     *time.Time                     `json:"revoked_at,omitempty"`
}

// TemplateDTO represents a credential template definition.
type TemplateDTO struct {
	ID                  string           `json:"id"`
	DriverID            string           `json:"driver_id"`
	Version             string           `json:"version"`
	DisplayName         string           `json:"display_name"`
	Description         string           `json:"description,omitempty"`
	Fields              []map[string]any `json:"fields"`
	CompatibleProtocols []string         `json:"compatible_protocols"`
	DeprecatedAfter     *time.Time       `json:"deprecated_after,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
	Hash                string           `json:"hash"`
}

// VaultCleanupResult summarises maintenance outcomes.
type VaultCleanupResult struct {
	OrphanedIdentities int
	ExpiredShares      int
}

// NewVaultService constructs a VaultService instance using the supplied dependencies.
func NewVaultService(db *gorm.DB, audit *AuditService, checker PermissionChecker, crypto *vault.Crypto) (*VaultService, error) {
	if db == nil {
		return nil, errors.New("vault service: db is required")
	}
	if crypto == nil {
		return nil, errors.New("vault service: crypto is required")
	}
	return &VaultService{
		db:      db,
		audit:   audit,
		checker: checker,
		crypto:  crypto,
	}, nil
}

// ResolveViewer builds a ViewerContext for the supplied user, fetching team memberships when needed.
func (s *VaultService) ResolveViewer(ctx context.Context, userID string, isRoot bool) (ViewerContext, error) {
	viewer := ViewerContext{
		UserID: strings.TrimSpace(userID),
		IsRoot: isRoot,
	}
	if viewer.UserID == "" {
		return viewer, nil
	}

	ctx = ensureContext(ctx)
	if isRoot {
		return viewer, nil
	}

	var teamIDs []string
	if err := s.db.WithContext(ctx).
		Table("user_teams").
		Where("user_id = ?", viewer.UserID).
		Pluck("team_id", &teamIDs).Error; err != nil {
		return ViewerContext{}, fmt.Errorf("vault service: resolve viewer teams: %w", err)
	}
	viewer.TeamIDs = teamIDs
	return viewer, nil
}

// ListIdentities returns identities visible to the supplied viewer.
func (s *VaultService) ListIdentities(ctx context.Context, viewer ViewerContext, opts ListIdentitiesOptions) ([]IdentityDTO, error) {
	ctx = ensureContext(ctx)

	query := s.db.WithContext(ctx).
		Model(&models.Identity{}).
		Select("DISTINCT identities.*").
		Preload("Shares", "revoked_at IS NULL")

	if !viewer.IsRoot {
		now := time.Now().UTC()
		query = query.Joins("LEFT JOIN identity_shares ON identity_shares.identity_id = identities.id AND identity_shares.revoked_at IS NULL")

		orClauses := []string{"identities.owner_user_id = ?"}
		args := []any{viewer.UserID}

		if len(viewer.TeamIDs) > 0 {
			orClauses = append(orClauses, "(identities.scope = ? AND identities.team_id IN ?)")
			args = append(args, models.IdentityScopeTeam, viewer.TeamIDs)
		}

		orClauses = append(orClauses, "(identity_shares.principal_type = ? AND identity_shares.principal_id = ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?))")
		args = append(args, models.IdentitySharePrincipalUser, viewer.UserID, now)

		if len(viewer.TeamIDs) > 0 {
			orClauses = append(orClauses, "(identity_shares.principal_type = ? AND identity_shares.principal_id IN ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?))")
			args = append(args, models.IdentitySharePrincipalTeam, viewer.TeamIDs, now)
		}

		query = query.Where(strings.Join(orClauses, " OR "), args...)
	}

	if opts.Scope != "" {
		query = query.Where("identities.scope = ?", opts.Scope)
	}

	if !opts.IncludeConnectionScoped {
		query = query.Where("identities.scope <> ?", models.IdentityScopeConnection)
	}

	var rows []models.Identity
	if err := query.Order("identities.created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("vault service: list identities: %w", err)
	}

	counts, err := s.connectionCounts(ctx, rows)
	if err != nil {
		return nil, err
	}

	results := make([]IdentityDTO, 0, len(rows))
	for _, row := range rows {
		dto := mapIdentity(row, nil, true, counts[row.ID])
		results = append(results, dto)
	}

	if strings.TrimSpace(opts.ProtocolID) != "" {
		results = s.filterByProtocol(ctx, results, strings.TrimSpace(opts.ProtocolID))
	}

	return results, nil
}

// GetIdentity returns a single identity if the viewer can access it. When includePayload is true the decrypted secret is included.
func (s *VaultService) GetIdentity(ctx context.Context, viewer ViewerContext, identityID string, includePayload bool) (IdentityDTO, error) {
	ctx = ensureContext(ctx)

	id := strings.TrimSpace(identityID)
	if id == "" {
		return IdentityDTO{}, apperrors.NewBadRequest("identity id is required")
	}

	query := s.db.WithContext(ctx).
		Model(&models.Identity{}).
		Preload("Shares", "revoked_at IS NULL")

	if !viewer.IsRoot {
		now := time.Now().UTC()
		query = query.Joins("LEFT JOIN identity_shares ON identity_shares.identity_id = identities.id AND identity_shares.revoked_at IS NULL").
			Where("identities.id = ?", id).
			Where(
				"(identities.owner_user_id = ?) OR "+
					"(identity_shares.principal_type = ? AND identity_shares.principal_id = ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?)) OR "+
					"(identity_shares.principal_type = ? AND identity_shares.principal_id IN ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?)) OR "+
					"(identities.scope = ? AND identities.team_id IN ?)",
				viewer.UserID,
				models.IdentitySharePrincipalUser, viewer.UserID, now,
				models.IdentitySharePrincipalTeam, viewer.TeamIDs, now,
				models.IdentityScopeTeam, viewer.TeamIDs,
			)
	} else {
		query = query.Where("identities.id = ?", id)
	}

	var identity models.Identity
	if err := query.First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return IdentityDTO{}, ErrIdentityNotFound
		}
		return IdentityDTO{}, fmt.Errorf("vault service: get identity: %w", err)
	}

	var payload map[string]any
	if includePayload {
		resultLabel := "denied"
		defer func() {
			monitoring.RecordVaultPayloadRequest(resultLabel)
		}()

		perm, ok := s.sharePermissionForViewer(viewer, identity)
		if !ok || perm != models.IdentitySharePermissionEdit {
			return IdentityDTO{}, apperrors.ErrForbidden
		}

		secret, err := s.decryptPayload(identity.EncryptedPayload)
		if err != nil {
			resultLabel = "error"
			return IdentityDTO{}, err
		}
		payload = secret

		now := time.Now().UTC()
		update := map[string]any{
			"usage_count":  gorm.Expr("usage_count + ?", 1),
			"last_used_at": now,
			"updated_at":   now,
		}
		if err := s.db.WithContext(ctx).Model(&models.Identity{}).
			Where("id = ?", identity.ID).
			UpdateColumns(update).Error; err != nil {
			resultLabel = "error"
			return IdentityDTO{}, fmt.Errorf("vault service: update usage stats: %w", err)
		}
		identity.UsageCount++
		identity.LastUsedAt = &now
		resultLabel = "allowed"
	}

	count, err := s.connectionCount(ctx, identity.ID)
	if err != nil {
		return IdentityDTO{}, err
	}

	dto := mapIdentity(identity, payload, true, count)
	return dto, nil
}

// AuthorizeIdentityUse verifies the viewer can access the requested identity and returns the raw model.
func (s *VaultService) AuthorizeIdentityUse(ctx context.Context, viewer ViewerContext, identityID string) (models.Identity, error) {
	ctx = ensureContext(ctx)

	id := strings.TrimSpace(identityID)
	if id == "" {
		return models.Identity{}, apperrors.NewBadRequest("identity id is required")
	}

	query := s.db.WithContext(ctx).
		Model(&models.Identity{}).
		Preload("Shares", "revoked_at IS NULL")
	if !viewer.IsRoot {
		now := time.Now().UTC()
		query = query.Joins("LEFT JOIN identity_shares ON identity_shares.identity_id = identities.id AND identity_shares.revoked_at IS NULL").
			Where("identities.id = ?", id).
			Where(
				"(identities.owner_user_id = ?) OR "+
					"(identity_shares.principal_type = ? AND identity_shares.principal_id = ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?)) OR "+
					"(identity_shares.principal_type = ? AND identity_shares.principal_id IN ? AND (identity_shares.expires_at IS NULL OR identity_shares.expires_at > ?)) OR "+
					"(identities.scope = ? AND identities.team_id IN ?)",
				viewer.UserID,
				models.IdentitySharePrincipalUser, viewer.UserID, now,
				models.IdentitySharePrincipalTeam, viewer.TeamIDs, now,
				models.IdentityScopeTeam, viewer.TeamIDs,
			)
	} else {
		query = query.Where("identities.id = ?", id)
	}

	var identity models.Identity
	if err := query.First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Identity{}, ErrIdentityNotFound
		}
		return models.Identity{}, fmt.Errorf("vault service: authorize identity: %w", err)
	}

	perm, ok := s.sharePermissionForViewer(viewer, identity)
	if !ok {
		return models.Identity{}, ErrIdentityNotFound
	}
	if perm != models.IdentitySharePermissionUse && perm != models.IdentitySharePermissionEdit {
		return models.Identity{}, apperrors.ErrForbidden
	}

	return identity, nil
}

// CreateIdentity stores a new encrypted identity and returns the persisted record.
func (s *VaultService) CreateIdentity(ctx context.Context, viewer ViewerContext, input CreateIdentityInput) (dto IdentityDTO, err error) {
	ctx = ensureContext(ctx)

	defer func() {
		monitoring.RecordVaultOperation("identity_create", operationResult(err))
	}()

	if !viewer.IsRoot {
		var ok bool
		if ok, err = s.checkPermission(ctx, viewer.UserID, "vault.create"); err != nil {
			return IdentityDTO{}, err
		} else if !ok {
			err = apperrors.ErrForbidden
			return IdentityDTO{}, err
		}
	}

	if validationErr := validateIdentityInput(input); validationErr != nil {
		err = apperrors.NewBadRequest(validationErr.Error())
		return IdentityDTO{}, err
	}

	var payloadBytes []byte
	if payloadBytes, err = json.Marshal(input.Payload); err != nil {
		err = apperrors.NewBadRequest("invalid credential payload")
		return IdentityDTO{}, err
	}

	var ciphertext string
	if ciphertext, err = s.crypto.Encrypt(payloadBytes); err != nil {
		err = fmt.Errorf("vault service: encrypt payload: %w", err)
		return IdentityDTO{}, err
	}

	var metadataJSON datatypes.JSON
	if input.Metadata != nil {
		var encoded []byte
		if encoded, err = json.Marshal(input.Metadata); err != nil {
			err = apperrors.NewBadRequest("invalid metadata payload")
			return IdentityDTO{}, err
		}
		metadataJSON = datatypes.JSON(encoded)
	}

	identity := models.Identity{
		Name:             strings.TrimSpace(input.Name),
		Description:      strings.TrimSpace(input.Description),
		Scope:            input.Scope,
		OwnerUserID:      strings.TrimSpace(input.OwnerUserID),
		TeamID:           normalizeOptionalID(input.TeamID),
		ConnectionID:     normalizeOptionalID(input.ConnectionID),
		TemplateID:       normalizeOptionalID(input.TemplateID),
		Version:          1,
		EncryptedPayload: ciphertext,
		Metadata:         metadataJSON,
	}

	if err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&identity).Error; err != nil {
			return fmt.Errorf("vault service: create identity: %w", err)
		}

		version := models.CredentialVersion{
			IdentityID:       identity.ID,
			Version:          identity.Version,
			EncryptedPayload: identity.EncryptedPayload,
			Metadata:         identity.Metadata,
			CreatedBy:        strings.TrimSpace(input.CreatedBy),
		}
		if err := tx.Create(&version).Error; err != nil {
			return fmt.Errorf("vault service: create credential version: %w", err)
		}
		return nil
	}); err != nil {
		return IdentityDTO{}, err
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "vault.identity.created",
		Result:   "success",
		Resource: "identity:" + identity.ID,
		Metadata: map[string]any{
			"scope":         identity.Scope,
			"template_id":   identity.TemplateID,
			"team_id":       identity.TeamID,
			"connection_id": identity.ConnectionID,
		},
	})

	dto = mapIdentity(identity, nil, false, 0)
	return dto, nil
}

// BindIdentityToConnection associates a connection-scoped identity with a connection record.
func (s *VaultService) BindIdentityToConnection(ctx context.Context, identityID, connectionID string) error {
	ctx = ensureContext(ctx)
	identityID = strings.TrimSpace(identityID)
	connectionID = strings.TrimSpace(connectionID)
	if identityID == "" || connectionID == "" {
		return apperrors.NewBadRequest("identity id and connection id are required")
	}

	result := s.db.WithContext(ctx).Model(&models.Identity{}).
		Where("id = ? AND scope = ?", identityID, models.IdentityScopeConnection).
		Updates(map[string]any{
			"connection_id": connectionID,
			"updated_at":    time.Now().UTC(),
		})

	if result.Error != nil {
		return fmt.Errorf("vault service: bind identity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrIdentityNotFound
	}
	return nil
}

// UpdateIdentity mutates an existing identity metadata or payload.
func (s *VaultService) UpdateIdentity(ctx context.Context, viewer ViewerContext, identityID string, input UpdateIdentityInput) (updated IdentityDTO, err error) {
	ctx = ensureContext(ctx)

	defer func() {
		monitoring.RecordVaultOperation("identity_update", operationResult(err))
	}()

	id := strings.TrimSpace(identityID)
	if id == "" {
		err = apperrors.NewBadRequest("identity id is required")
		return IdentityDTO{}, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Identity{}).Where("id = ?", id).Preload("Shares", "revoked_at IS NULL")

		if !viewer.IsRoot {
			query = query.Where("owner_user_id = ?", viewer.UserID)
		}

		var identity models.Identity
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIdentityNotFound
			}
			return fmt.Errorf("vault service: update identity fetch: %w", err)
		}

		changes := map[string]any{}
		if input.Name != nil {
			identity.Name = strings.TrimSpace(*input.Name)
			changes["name"] = identity.Name
		}
		if input.Description != nil {
			identity.Description = strings.TrimSpace(*input.Description)
			changes["description"] = identity.Description
		}
		if input.TemplateID != nil {
			identity.TemplateID = normalizeOptionalID(input.TemplateID)
			changes["template_id"] = identity.TemplateID
		}
		if input.ConnectionID != nil {
			identity.ConnectionID = normalizeOptionalID(input.ConnectionID)
			changes["connection_id"] = identity.ConnectionID
		}

		if input.Metadata != nil {
			encoded, err := json.Marshal(input.Metadata)
			if err != nil {
				return apperrors.NewBadRequest("invalid metadata payload")
			}
			identity.Metadata = encoded
			changes["metadata"] = identity.Metadata
		}

		var payload map[string]any
		var lastRotatedAt *time.Time
		if input.Payload != nil {
			bytes, err := json.Marshal(input.Payload)
			if err != nil {
				return apperrors.NewBadRequest("invalid credential payload")
			}

			encrypted, err := s.crypto.Encrypt(bytes)
			if err != nil {
				return fmt.Errorf("vault service: encrypt payload: %w", err)
			}

			identity.Version++
			identity.EncryptedPayload = encrypted
			now := time.Now().UTC()
			identity.LastRotatedAt = &now
			lastRotatedAt = &now

			if err := tx.Create(&models.CredentialVersion{
				IdentityID:       identity.ID,
				Version:          identity.Version,
				EncryptedPayload: identity.EncryptedPayload,
				Metadata:         identity.Metadata,
				CreatedBy:        strings.TrimSpace(viewer.UserID),
			}).Error; err != nil {
				return fmt.Errorf("vault service: create credential version: %w", err)
			}

			changes["version"] = identity.Version
			changes["encrypted_payload"] = identity.EncryptedPayload
			changes["last_rotated_at"] = identity.LastRotatedAt
			payload = input.Payload
		}

		if len(changes) == 0 {
			updated = mapIdentity(identity, payload, true, 0)
			return nil
		}

		timestamp := time.Now().UTC()
		changes["updated_at"] = timestamp
		identity.UpdatedAt = timestamp
		if lastRotatedAt != nil {
			identity.LastRotatedAt = lastRotatedAt
		}
		if err := tx.Model(&identity).Updates(changes).Error; err != nil {
			return fmt.Errorf("vault service: persist identity updates: %w", err)
		}

		payload = nil
		updated = mapIdentity(identity, payload, true, 0)
		return nil
	})
	if err != nil {
		return IdentityDTO{}, err
	}

	var count int
	if count, err = s.connectionCount(ctx, updated.ID); err != nil {
		return IdentityDTO{}, err
	}
	updated.ConnectionCount = count

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "vault.identity.updated",
		Result:   "success",
		Resource: "identity:" + updated.ID,
		Metadata: map[string]any{
			"version": updated.Version,
		},
	})

	return updated, nil
}

// DeleteIdentity removes an identity and associated shares/versions.
func (s *VaultService) DeleteIdentity(ctx context.Context, viewer ViewerContext, identityID string) (err error) {
	ctx = ensureContext(ctx)

	defer func() {
		monitoring.RecordVaultOperation("identity_delete", operationResult(err))
	}()

	id := strings.TrimSpace(identityID)
	if id == "" {
		err = apperrors.NewBadRequest("identity id is required")
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Identity{}).Where("id = ?", id)
		if !viewer.IsRoot {
			query = query.Where("owner_user_id = ?", viewer.UserID)
		}

		var identity models.Identity
		if err := query.First(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIdentityNotFound
			}
			return fmt.Errorf("vault service: delete identity fetch: %w", err)
		}

		if err := tx.Where("identity_id = ?", identity.ID).Delete(&models.IdentityShare{}).Error; err != nil {
			return fmt.Errorf("vault service: delete shares: %w", err)
		}
		if err := tx.Where("identity_id = ?", identity.ID).Delete(&models.CredentialVersion{}).Error; err != nil {
			return fmt.Errorf("vault service: delete credential versions: %w", err)
		}
		if err := tx.Delete(&identity).Error; err != nil {
			return fmt.Errorf("vault service: delete identity: %w", err)
		}

		recordAudit(s.audit, ctx, AuditEntry{
			Action:   "vault.identity.deleted",
			Result:   "success",
			Resource: "identity:" + identity.ID,
			Metadata: map[string]any{
				"scope": identity.Scope,
			},
		})
		return nil
	})
	return err
}

// CreateShare grants access to an identity for the specified principal.
func (s *VaultService) CreateShare(ctx context.Context, viewer ViewerContext, identityID string, input IdentityShareInput) (dto IdentityShareDTO, err error) {
	ctx = ensureContext(ctx)

	defer func() {
		monitoring.RecordVaultOperation("identity_share_grant", operationResult(err))
	}()

	if !viewer.IsRoot {
		var ok bool
		if ok, err = s.checkPermission(ctx, viewer.UserID, "vault.share"); err != nil {
			return IdentityShareDTO{}, err
		} else if !ok {
			err = apperrors.ErrForbidden
			return IdentityShareDTO{}, err
		}
	}

	id := strings.TrimSpace(identityID)
	if id == "" {
		err = apperrors.NewBadRequest("identity id is required")
		return IdentityShareDTO{}, err
	}

	if validationErr := validateShareInput(input); validationErr != nil {
		err = apperrors.NewBadRequest(validationErr.Error())
		return IdentityShareDTO{}, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var identity models.Identity
		query := tx.Where("id = ?", id)
		if !viewer.IsRoot {
			query = query.Where("owner_user_id = ?", viewer.UserID)
		}
		if err := query.First(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIdentityNotFound
			}
			return fmt.Errorf("vault service: share identity fetch: %w", err)
		}

		var metadata datatypes.JSON
		if input.Metadata != nil {
			payload, err := json.Marshal(input.Metadata)
			if err != nil {
				return apperrors.NewBadRequest("invalid share metadata")
			}
			metadata = payload
		}

		share := models.IdentityShare{
			IdentityID:    identity.ID,
			PrincipalType: input.PrincipalType,
			PrincipalID:   strings.TrimSpace(input.PrincipalID),
			Permission:    input.Permission,
			ExpiresAt:     input.ExpiresAt,
			Metadata:      metadata,
			GrantedBy:     strings.TrimSpace(viewer.UserID),
			CreatedBy:     strings.TrimSpace(viewer.UserID),
			UpdatedBy:     strings.TrimSpace(viewer.UserID),
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "identity_id"}, {Name: "principal_type"}, {Name: "principal_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"permission", "expires_at", "metadata", "granted_by", "updated_by", "revoked_at", "revoked_by"}),
		}).Create(&share).Error; err != nil {
			return fmt.Errorf("vault service: create share: %w", err)
		}

		dto = mapShare(share)
		return nil
	})
	if err != nil {
		return IdentityShareDTO{}, err
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "vault.identity.shared",
		Result:   "success",
		Resource: "identity:" + identityID,
		Metadata: map[string]any{
			"principal_type": dto.PrincipalType,
			"principal_id":   dto.PrincipalID,
			"permission":     dto.Permission,
		},
	})

	return dto, nil
}

// DeleteShare revokes an identity share by ID.
func (s *VaultService) DeleteShare(ctx context.Context, viewer ViewerContext, shareID string) (err error) {
	ctx = ensureContext(ctx)

	defer func() {
		monitoring.RecordVaultOperation("identity_share_revoke", operationResult(err))
	}()

	id := strings.TrimSpace(shareID)
	if id == "" {
		err = apperrors.NewBadRequest("share id is required")
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var share models.IdentityShare
		if err := tx.First(&share, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrShareNotFound
			}
			return fmt.Errorf("vault service: delete share fetch: %w", err)
		}

		var identity models.Identity
		if err := tx.First(&identity, "id = ?", share.IdentityID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIdentityNotFound
			}
			return fmt.Errorf("vault service: delete share load identity: %w", err)
		}

		if !viewer.IsRoot && identity.OwnerUserID != viewer.UserID {
			return apperrors.ErrForbidden
		}

		if err := tx.Delete(&share).Error; err != nil {
			return fmt.Errorf("vault service: delete share: %w", err)
		}

		recordAudit(s.audit, ctx, AuditEntry{
			Action:   "vault.identity.share_revoked",
			Result:   "success",
			Resource: "identity:" + identity.ID,
			Metadata: map[string]any{
				"principal_type": share.PrincipalType,
				"principal_id":   share.PrincipalID,
			},
		})
		return nil
	})
	return err
}

// RevokeShareForPrincipal removes an identity share for the supplied principal if present.
func (s *VaultService) RevokeShareForPrincipal(ctx context.Context, viewer ViewerContext, identityID string, principalType models.IdentitySharePrincipal, principalID string) error {
	ctx = ensureContext(ctx)

	identityID = strings.TrimSpace(identityID)
	if identityID == "" {
		return apperrors.NewBadRequest("identity id is required")
	}

	var share models.IdentityShare
	if err := s.db.WithContext(ctx).
		First(&share, "identity_id = ? AND principal_type = ? AND principal_id = ?", identityID, principalType, principalID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("vault service: load share for revoke: %w", err)
	}

	return s.DeleteShare(ctx, viewer, share.ID)
}

// CleanupOrphans removes expired shares and connection-scoped identities without backing connections.
func (s *VaultService) CleanupOrphans(ctx context.Context) (VaultCleanupResult, error) {
	ctx = ensureContext(ctx)

	result := VaultCleanupResult{}

	now := time.Now().UTC()
	expired := s.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", now).
		Delete(&models.IdentityShare{})
	if expired.Error != nil {
		return result, fmt.Errorf("vault service: delete expired shares: %w", expired.Error)
	}
	result.ExpiredShares = int(expired.RowsAffected)

	var orphanIDs []string
	if err := s.db.WithContext(ctx).
		Table("identities").
		Select("identities.id").
		Joins("LEFT JOIN connections ON connections.identity_id = identities.id").
		Where("identities.scope = ?", models.IdentityScopeConnection).
		Where("connections.id IS NULL OR identities.connection_id IS NULL").
		Pluck("identities.id", &orphanIDs).Error; err != nil {
		return result, fmt.Errorf("vault service: locate orphan identities: %w", err)
	}

	if len(orphanIDs) == 0 {
		return result, nil
	}

	if err := s.db.WithContext(ctx).Where("identity_id IN ?", orphanIDs).Delete(&models.IdentityShare{}).Error; err != nil {
		return result, fmt.Errorf("vault service: delete orphan shares: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("identity_id IN ?", orphanIDs).Delete(&models.CredentialVersion{}).Error; err != nil {
		return result, fmt.Errorf("vault service: delete credential versions: %w", err)
	}
	identities := s.db.WithContext(ctx).Where("id IN ?", orphanIDs).Delete(&models.Identity{})
	if identities.Error != nil {
		return result, fmt.Errorf("vault service: delete orphan identities: %w", identities.Error)
	}
	result.OrphanedIdentities = int(identities.RowsAffected)
	return result, nil
}

// ListTemplates returns all credential templates stored in the catalog.
func (s *VaultService) ListTemplates(ctx context.Context) ([]TemplateDTO, error) {
	ctx = ensureContext(ctx)

	var rows []models.CredentialTemplate
	if err := s.db.WithContext(ctx).
		Order("driver_id ASC, version DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("vault service: list templates: %w", err)
	}

	results := make([]TemplateDTO, 0, len(rows))
	for _, tpl := range rows {
		fields, err := decodeJSONSlice(tpl.Fields)
		if err != nil {
			return nil, fmt.Errorf("vault service: decode template fields: %w", err)
		}
		protocols := decodeJSONStrings(tpl.CompatibleProtocols)
		meta := decodeJSONMap(tpl.Metadata)
		results = append(results, TemplateDTO{
			ID:                  tpl.ID,
			DriverID:            tpl.DriverID,
			Version:             tpl.Version,
			DisplayName:         tpl.DisplayName,
			Description:         tpl.Description,
			Fields:              fields,
			CompatibleProtocols: protocols,
			DeprecatedAfter:     tpl.DeprecatedAfter,
			Metadata:            meta,
			Hash:                tpl.Hash,
		})
	}

	return results, nil
}

func (s *VaultService) filterByProtocol(ctx context.Context, identities []IdentityDTO, protocolID string) []IdentityDTO {
	if len(identities) == 0 {
		return identities
	}

	templateIDs := make([]string, 0, len(identities))
	for _, identity := range identities {
		if identity.TemplateID != nil {
			templateIDs = append(templateIDs, *identity.TemplateID)
		}
	}
	if len(templateIDs) == 0 {
		return []IdentityDTO{}
	}

	var templates []models.CredentialTemplate
	if err := s.db.WithContext(ctx).
		Where("id IN ?", templateIDs).
		Find(&templates).Error; err != nil {
		return identities
	}

	compatible := map[string]bool{}
	for _, tpl := range templates {
		protocols := decodeJSONStrings(tpl.CompatibleProtocols)
		for _, proto := range protocols {
			if strings.EqualFold(proto, protocolID) {
				compatible[tpl.ID] = true
				break
			}
		}
	}

	filtered := make([]IdentityDTO, 0, len(identities))
	for _, identity := range identities {
		if identity.TemplateID != nil && compatible[*identity.TemplateID] {
			filtered = append(filtered, identity)
		}
	}
	return filtered
}

func (s *VaultService) decryptPayload(ciphertext string) (map[string]any, error) {
	if strings.TrimSpace(ciphertext) == "" {
		return nil, apperrors.NewBadRequest("identity has no payload")
	}

	bytes, err := s.crypto.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("vault service: decrypt payload: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return nil, fmt.Errorf("vault service: decode payload: %w", err)
	}
	return payload, nil
}

func (s *VaultService) checkPermission(ctx context.Context, userID, permission string) (bool, error) {
	if s.checker == nil {
		return true, nil
	}
	return s.checker.Check(ctx, strings.TrimSpace(userID), permission)
}

func (s *VaultService) connectionCounts(ctx context.Context, identities []models.Identity) (map[string]int, error) {
	counts := make(map[string]int, len(identities))
	if len(identities) == 0 {
		return counts, nil
	}

	ids := make([]string, 0, len(identities))
	for _, identity := range identities {
		counts[identity.ID] = 0
		ids = append(ids, identity.ID)
	}

	var rows []struct {
		IdentityID string
		Count      int64
	}

	if err := s.db.WithContext(ctx).
		Table("connections").
		Select("identity_id, COUNT(*) AS count").
		Where("identity_id IN ?", ids).
		Group("identity_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("vault service: load connection counts: %w", err)
	}

	for _, row := range rows {
		counts[row.IdentityID] = int(row.Count)
	}

	return counts, nil
}

func (s *VaultService) connectionCount(ctx context.Context, identityID string) (int, error) {
	identityID = strings.TrimSpace(identityID)
	if identityID == "" {
		return 0, nil
	}

	var total int64
	if err := s.db.WithContext(ctx).
		Model(&models.Connection{}).
		Where("identity_id = ?", identityID).
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("vault service: count identity connections: %w", err)
	}
	return int(total), nil
}

var sharePermissionPriority = map[models.IdentitySharePermission]int{
	models.IdentitySharePermissionUse:          1,
	models.IdentitySharePermissionViewMetadata: 2,
	models.IdentitySharePermissionEdit:         3,
}

func (s *VaultService) sharePermissionForViewer(viewer ViewerContext, identity models.Identity) (models.IdentitySharePermission, bool) {
	if viewer.IsRoot || strings.TrimSpace(identity.OwnerUserID) == strings.TrimSpace(viewer.UserID) {
		return models.IdentitySharePermissionEdit, true
	}

	if len(identity.Shares) == 0 {
		return "", false
	}

	now := time.Now().UTC()
	highest := 0
	var resolved models.IdentitySharePermission

	for _, share := range identity.Shares {
		if share.RevokedAt != nil {
			continue
		}
		if share.ExpiresAt != nil && now.After(*share.ExpiresAt) {
			continue
		}

		switch share.PrincipalType {
		case models.IdentitySharePrincipalUser:
			if strings.TrimSpace(share.PrincipalID) != strings.TrimSpace(viewer.UserID) {
				continue
			}
		case models.IdentitySharePrincipalTeam:
			if !containsString(viewer.TeamIDs, share.PrincipalID) {
				continue
			}
		default:
			continue
		}

		if rank, ok := sharePermissionPriority[share.Permission]; ok && rank > highest {
			highest = rank
			resolved = share.Permission
		}
	}

	if highest == 0 {
		return "", false
	}
	return resolved, true
}

func operationResult(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

func mapIdentity(identity models.Identity, payload map[string]any, includeShares bool, connectionCount int) IdentityDTO {
	dto := IdentityDTO{
		ID:              identity.ID,
		Name:            identity.Name,
		Description:     identity.Description,
		Scope:           identity.Scope,
		OwnerUserID:     identity.OwnerUserID,
		TeamID:          identity.TeamID,
		ConnectionID:    identity.ConnectionID,
		TemplateID:      identity.TemplateID,
		Version:         identity.Version,
		Metadata:        decodeJSONMap(identity.Metadata),
		UsageCount:      identity.UsageCount,
		LastUsedAt:      identity.LastUsedAt,
		LastRotatedAt:   identity.LastRotatedAt,
		CreatedAt:       identity.CreatedAt,
		UpdatedAt:       identity.UpdatedAt,
		Payload:         payload,
		ConnectionCount: connectionCount,
	}

	if includeShares && len(identity.Shares) > 0 {
		shares := make([]IdentityShareDTO, 0, len(identity.Shares))
		for _, share := range identity.Shares {
			if share.RevokedAt != nil {
				continue
			}
			shares = append(shares, mapShare(share))
		}
		dto.Shares = shares
	}

	return dto
}

func mapShare(share models.IdentityShare) IdentityShareDTO {
	return IdentityShareDTO{
		ID:            share.ID,
		PrincipalType: share.PrincipalType,
		PrincipalID:   share.PrincipalID,
		Permission:    share.Permission,
		ExpiresAt:     share.ExpiresAt,
		Metadata:      decodeJSONMap(share.Metadata),
		GrantedBy:     share.GrantedBy,
		CreatedBy:     share.CreatedBy,
		RevokedBy:     share.RevokedBy,
		RevokedAt:     share.RevokedAt,
	}
}

func decodeJSONSlice(data datatypes.JSON) ([]map[string]any, error) {
	if len(data) == 0 {
		return []map[string]any{}, nil
	}
	var result []map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeJSONStrings(data datatypes.JSON) []string {
	if len(data) == 0 {
		return nil
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func validateIdentityInput(input CreateIdentityInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("identity name is required")
	}
	if strings.TrimSpace(input.OwnerUserID) == "" {
		return errors.New("owner user id is required")
	}
	if input.Payload == nil {
		return errors.New("credential payload is required")
	}

	scope := models.IdentityScope(strings.TrimSpace(string(input.Scope)))
	switch scope {
	case models.IdentityScopeGlobal:
		input.TeamID = nil
		input.ConnectionID = nil
	case models.IdentityScopeTeam:
		if input.TeamID == nil || strings.TrimSpace(*input.TeamID) == "" {
			return errors.New("team id is required for team scoped identities")
		}
	case models.IdentityScopeConnection:
		if input.ConnectionID == nil || strings.TrimSpace(*input.ConnectionID) == "" {
			return errors.New("connection id is required for connection scoped identities")
		}
	default:
		return fmt.Errorf("invalid identity scope %q", input.Scope)
	}
	return nil
}

func validateShareInput(input IdentityShareInput) error {
	if strings.TrimSpace(input.PrincipalID) == "" {
		return errors.New("principal id is required")
	}
	switch input.PrincipalType {
	case models.IdentitySharePrincipalUser, models.IdentitySharePrincipalTeam:
	default:
		return errors.New("invalid principal type")
	}
	switch input.Permission {
	case models.IdentitySharePermissionUse,
		models.IdentitySharePermissionViewMetadata,
		models.IdentitySharePermissionEdit:
	default:
		return errors.New("invalid permission")
	}
	return nil
}
