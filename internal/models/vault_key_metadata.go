package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// VaultKeyMetadata captures information about derived vault encryption keys.
type VaultKeyMetadata struct {
	BaseModel

	KeyID         string         `gorm:"not null;uniqueIndex" json:"key_id"`
	IsActive      bool           `gorm:"not null;default:false;index" json:"is_active"`
	KDFAlgorithm  string         `gorm:"not null" json:"kdf_algorithm"`
	KDFParameters datatypes.JSON `gorm:"not null" json:"kdf_parameters"`
	Salt          []byte         `gorm:"not null" json:"salt"`
	DerivedAt     time.Time      `gorm:"not null" json:"derived_at"`
	MaterialHash  string         `gorm:"not null" json:"material_hash"`
	RotatedAt     *time.Time     `json:"rotated_at"`
	Notes         string         `json:"notes"`
}

// BeforeSave validates key metadata consistency.
func (m *VaultKeyMetadata) BeforeSave(tx *gorm.DB) error {
	m.KeyID = strings.TrimSpace(m.KeyID)
	if m.KeyID == "" {
		return errors.New("vault_key_metadata: key_id is required")
	}
	m.KDFAlgorithm = strings.TrimSpace(m.KDFAlgorithm)
	if m.KDFAlgorithm == "" {
		return errors.New("vault_key_metadata: kdf_algorithm is required")
	}
	if len(m.Salt) < 16 {
		return errors.New("vault_key_metadata: salt must be at least 16 bytes")
	}
	if len(m.KDFParameters) == 0 {
		return errors.New("vault_key_metadata: kdf_parameters is required")
	}
	m.MaterialHash = strings.TrimSpace(m.MaterialHash)
	if m.MaterialHash == "" {
		return errors.New("vault_key_metadata: material_hash is required")
	}
	if m.DerivedAt.IsZero() {
		m.DerivedAt = time.Now().UTC()
	}
	return nil
}
