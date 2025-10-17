package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ConnectionTemplate captures the schema required to render protocol-specific connection fields.
type ConnectionTemplate struct {
	BaseModel

	DriverID    string         `gorm:"not null;uniqueIndex:idx_connection_template_driver_version,priority:1" json:"driver_id"`
	Version     string         `gorm:"not null;uniqueIndex:idx_connection_template_driver_version,priority:2" json:"version"`
	DisplayName string         `gorm:"not null" json:"display_name"`
	Description string         `json:"description"`
	Sections    datatypes.JSON `gorm:"not null" json:"sections"`
	Protocols   datatypes.JSON `json:"protocols"`
	Metadata    datatypes.JSON `json:"metadata"`
	Hash        string         `gorm:"index" json:"hash"`
}

// BeforeSave validates connection template invariants to avoid persisting malformed schemas.
func (t *ConnectionTemplate) BeforeSave(tx *gorm.DB) error {
	t.DriverID = strings.TrimSpace(t.DriverID)
	if t.DriverID == "" {
		return errors.New("connection_template: driver_id is required")
	}

	t.Version = strings.TrimSpace(t.Version)
	if t.Version == "" {
		return errors.New("connection_template: version is required")
	}

	t.DisplayName = strings.TrimSpace(t.DisplayName)
	if t.DisplayName == "" {
		return errors.New("connection_template: display_name is required")
	}

	if len(t.Sections) == 0 {
		return errors.New("connection_template: sections must not be empty")
	}

	var rawSections []map[string]any
	if err := json.Unmarshal(t.Sections, &rawSections); err != nil {
		return fmt.Errorf("connection_template: sections must be valid json: %w", err)
	}
	if len(rawSections) == 0 {
		return errors.New("connection_template: at least one section is required")
	}
	for i, section := range rawSections {
		fieldsRaw, ok := section["fields"].([]any)
		if !ok || len(fieldsRaw) == 0 {
			return fmt.Errorf("connection_template: section %d has no fields", i)
		}
	}

	var rawProtocols []string
	if len(t.Protocols) > 0 {
		if err := json.Unmarshal(t.Protocols, &rawProtocols); err != nil {
			return fmt.Errorf("connection_template: protocols must be valid json: %w", err)
		}
	}

	normalized := NormalizeConnectionTemplateProtocols(rawProtocols, t.DriverID)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("connection_template: marshal protocols: %w", err)
	}
	t.Protocols = datatypes.JSON(data)

	return nil
}

// NormalizeConnectionTemplateProtocols deduplicates and normalizes protocol identifiers.
func NormalizeConnectionTemplateProtocols(protocols []string, driverID string) []string {
	driverID = strings.TrimSpace(strings.ToLower(driverID))
	seen := make(map[string]struct{})
	ordered := make([]string, 0, len(protocols)+1)

	if driverID != "" {
		seen[driverID] = struct{}{}
		ordered = append(ordered, driverID)
	}

	for _, id := range protocols {
		normalized := strings.TrimSpace(strings.ToLower(id))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		ordered = append(ordered, normalized)
	}

	if len(ordered) == 0 && driverID != "" {
		return []string{driverID}
	}

	return ordered
}
