package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CredentialTemplate captures the schema needed to render credential forms.
type CredentialTemplate struct {
	BaseModel

	DriverID            string         `gorm:"not null;uniqueIndex:idx_credential_template_driver_version,priority:1" json:"driver_id"`
	Version             string         `gorm:"not null;uniqueIndex:idx_credential_template_driver_version,priority:2" json:"version"`
	DisplayName         string         `gorm:"not null" json:"display_name"`
	Description         string         `json:"description"`
	Fields              datatypes.JSON `gorm:"not null" json:"fields"`
	CompatibleProtocols datatypes.JSON `gorm:"not null" json:"compatible_protocols"`
	DeprecatedAfter     *time.Time     `json:"deprecated_after"`
	Metadata            datatypes.JSON `json:"metadata"`
	Hash                string         `gorm:"index" json:"hash"`
}

// BeforeSave ensures the template metadata is valid.
func (t *CredentialTemplate) BeforeSave(tx *gorm.DB) error {
	t.DriverID = strings.TrimSpace(t.DriverID)
	if t.DriverID == "" {
		return errors.New("credential_template: driver_id is required")
	}

	t.Version = strings.TrimSpace(t.Version)
	if t.Version == "" {
		return errors.New("credential_template: version is required")
	}

	t.DisplayName = strings.TrimSpace(t.DisplayName)
	if t.DisplayName == "" {
		return errors.New("credential_template: display_name is required")
	}

	if len(t.Fields) == 0 {
		return errors.New("credential_template: fields must not be empty")
	}

	if len(t.CompatibleProtocols) == 0 {
		return fmt.Errorf("credential_template: compatible_protocols must not be empty for driver %s", t.DriverID)
	}

	return nil
}
