package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IdentityScope enumerates the supported identity scopes.
type IdentityScope string

const (
	// IdentityScopeGlobal identities can be reused by any resource the owner can access.
	IdentityScopeGlobal IdentityScope = "global"
	// IdentityScopeTeam identities are shared with a specific team.
	IdentityScopeTeam IdentityScope = "team"
	// IdentityScopeConnection identities are bound to a single connection.
	IdentityScopeConnection IdentityScope = "connection"
)

var validIdentityScopes = map[IdentityScope]struct{}{
	IdentityScopeGlobal:     {},
	IdentityScopeTeam:       {},
	IdentityScopeConnection: {},
}

// Identity represents an encrypted credential payload managed by the vault.
type Identity struct {
	BaseModel

	Name             string         `gorm:"not null;index" json:"name"`
	Description      string         `json:"description"`
	Scope            IdentityScope  `gorm:"type:text;not null;index" json:"scope"`
	OwnerUserID      string         `gorm:"type:uuid;not null;index" json:"owner_user_id"`
	TeamID           *string        `gorm:"type:uuid;index" json:"team_id"`
	ConnectionID     *string        `gorm:"type:uuid;index" json:"connection_id"`
	TemplateID       *string        `gorm:"type:uuid;index" json:"template_id"`
	Version          int            `gorm:"not null;default:1" json:"version"`
	EncryptedPayload string         `gorm:"type:text;not null" json:"encrypted_payload"`
	Metadata         datatypes.JSON `json:"metadata"`
	UsageCount       int            `gorm:"not null;default:0" json:"usage_count"`
	LastUsedAt       *time.Time     `json:"last_used_at"`
	LastRotatedAt    *time.Time     `json:"last_rotated_at"`

	Connection *Connection         `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
	Shares     []IdentityShare     `gorm:"foreignKey:IdentityID" json:"shares,omitempty"`
	Versions   []CredentialVersion `gorm:"foreignKey:IdentityID" json:"versions,omitempty"`
}

// BeforeSave validates scope relationships and normalises identity metadata.
func (i *Identity) BeforeSave(tx *gorm.DB) error {
	i.Name = strings.TrimSpace(i.Name)
	if i.Name == "" {
		return errors.New("identity: name is required")
	}

	i.OwnerUserID = strings.TrimSpace(i.OwnerUserID)
	if i.OwnerUserID == "" {
		return errors.New("identity: owner_user_id is required")
	}

	scope := IdentityScope(strings.TrimSpace(string(i.Scope)))
	if _, ok := validIdentityScopes[scope]; !ok {
		return fmt.Errorf("identity: invalid scope %q", i.Scope)
	}
	i.Scope = scope

	switch scope {
	case IdentityScopeGlobal:
		i.TeamID = nil
		i.ConnectionID = nil
	case IdentityScopeTeam:
		if i.TeamID == nil || strings.TrimSpace(*i.TeamID) == "" {
			return errors.New("identity: team_id is required for team scope identities")
		}
		i.ConnectionID = nil
	case IdentityScopeConnection:
		if i.TeamID != nil {
			return errors.New("identity: team_id must be nil for connection scoped identities")
		}
	}

	i.EncryptedPayload = strings.TrimSpace(i.EncryptedPayload)
	if i.EncryptedPayload == "" {
		return errors.New("identity: encrypted_payload is required")
	}

	if i.Version <= 0 {
		i.Version = 1
	}

	return nil
}
