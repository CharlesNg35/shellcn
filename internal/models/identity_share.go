package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IdentitySharePermission enumerates the supported share permission levels.
type IdentitySharePermission string

const (
	// IdentitySharePermissionUse allows using the identity for launches.
	IdentitySharePermissionUse IdentitySharePermission = "use"
	// IdentitySharePermissionViewMetadata allows viewing metadata without decrypting credentials.
	IdentitySharePermissionViewMetadata IdentitySharePermission = "view_metadata"
	// IdentitySharePermissionEdit allows editing the identity.
	IdentitySharePermissionEdit IdentitySharePermission = "edit"
)

var validSharePermissions = map[IdentitySharePermission]struct{}{
	IdentitySharePermissionUse:          {},
	IdentitySharePermissionViewMetadata: {},
	IdentitySharePermissionEdit:         {},
}

// IdentitySharePrincipal enumerates supported share principals.
type IdentitySharePrincipal string

const (
	// IdentitySharePrincipalUser indicates a user share.
	IdentitySharePrincipalUser IdentitySharePrincipal = "user"
	// IdentitySharePrincipalTeam indicates a team share.
	IdentitySharePrincipalTeam IdentitySharePrincipal = "team"
)

var validSharePrincipals = map[IdentitySharePrincipal]struct{}{
	IdentitySharePrincipalUser: {},
	IdentitySharePrincipalTeam: {},
}

// IdentityShare tracks permissions granted to principals for an identity.
type IdentityShare struct {
	BaseModel

	IdentityID    string                  `gorm:"type:uuid;not null;uniqueIndex:idx_identity_share_principal,priority:0;index" json:"identity_id"`
	PrincipalType IdentitySharePrincipal  `gorm:"type:text;not null;uniqueIndex:idx_identity_share_principal,priority:1" json:"principal_type"`
	PrincipalID   string                  `gorm:"type:text;not null;uniqueIndex:idx_identity_share_principal,priority:2" json:"principal_id"`
	Permission    IdentitySharePermission `gorm:"type:text;not null" json:"permission"`
	ExpiresAt     *time.Time              `json:"expires_at"`
	Metadata      datatypes.JSON          `json:"metadata"`
	GrantedBy     string                  `gorm:"type:uuid;not null" json:"granted_by"`
	RevokedBy     *string                 `gorm:"type:uuid" json:"revoked_by"`
	RevokedAt     *time.Time              `json:"revoked_at"`
	CreatedBy     string                  `gorm:"type:uuid;not null" json:"created_by"`
	UpdatedBy     string                  `gorm:"type:uuid;not null" json:"updated_by"`
	Identity      *Identity               `gorm:"foreignKey:IdentityID" json:"identity,omitempty"`
}

// BeforeSave validates principal and permission metadata.
func (s *IdentityShare) BeforeSave(tx *gorm.DB) error {
	s.IdentityID = strings.TrimSpace(s.IdentityID)
	if s.IdentityID == "" {
		return errors.New("identity_share: identity_id is required")
	}

	s.PrincipalID = strings.TrimSpace(s.PrincipalID)
	if s.PrincipalID == "" {
		return errors.New("identity_share: principal_id is required")
	}

	perm := IdentitySharePermission(strings.TrimSpace(string(s.Permission)))
	if _, ok := validSharePermissions[perm]; !ok {
		return fmt.Errorf("identity_share: invalid permission %q", s.Permission)
	}
	s.Permission = perm

	principal := IdentitySharePrincipal(strings.TrimSpace(string(s.PrincipalType)))
	if _, ok := validSharePrincipals[principal]; !ok {
		return fmt.Errorf("identity_share: invalid principal type %q", s.PrincipalType)
	}
	s.PrincipalType = principal

	s.CreatedBy = strings.TrimSpace(s.CreatedBy)
	if s.CreatedBy == "" {
		return errors.New("identity_share: created_by is required")
	}

	s.UpdatedBy = strings.TrimSpace(s.UpdatedBy)
	if s.UpdatedBy == "" {
		s.UpdatedBy = s.CreatedBy
	}

	s.GrantedBy = strings.TrimSpace(s.GrantedBy)
	if s.GrantedBy == "" {
		s.GrantedBy = s.CreatedBy
	}

	return nil
}
