package models

import (
	"errors"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CredentialVersion stores historical encrypted payloads for an identity.
type CredentialVersion struct {
	BaseModel

	IdentityID       string         `gorm:"type:uuid;not null;index:idx_credential_version_identity_version,priority:1" json:"identity_id"`
	Version          int            `gorm:"not null;index:idx_credential_version_identity_version,priority:2" json:"version"`
	EncryptedPayload string         `gorm:"type:text;not null" json:"encrypted_payload"`
	Metadata         datatypes.JSON `json:"metadata"`
	CreatedBy        string         `gorm:"type:uuid;not null" json:"created_by"`

	Identity *Identity `gorm:"foreignKey:IdentityID" json:"identity,omitempty"`
}

// BeforeSave validates credential version metadata.
func (v *CredentialVersion) BeforeSave(tx *gorm.DB) error {
	v.IdentityID = strings.TrimSpace(v.IdentityID)
	if v.IdentityID == "" {
		return errors.New("credential_version: identity_id is required")
	}
	if v.Version <= 0 {
		return errors.New("credential_version: version must be greater than zero")
	}
	v.EncryptedPayload = strings.TrimSpace(v.EncryptedPayload)
	if v.EncryptedPayload == "" {
		return errors.New("credential_version: encrypted_payload is required")
	}
	v.CreatedBy = strings.TrimSpace(v.CreatedBy)
	if v.CreatedBy == "" {
		return errors.New("credential_version: created_by is required")
	}
	return nil
}
