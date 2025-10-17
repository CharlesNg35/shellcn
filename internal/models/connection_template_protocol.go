package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ConnectionTemplateProtocol maps protocols to the driver template version that materialises them.
type ConnectionTemplateProtocol struct {
	ProtocolID string    `gorm:"primaryKey" json:"protocol_id"`
	DriverID   string    `gorm:"not null" json:"driver_id"`
	Version    string    `gorm:"not null" json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// BeforeSave ensures protocol entries are valid prior to persistence.
func (p *ConnectionTemplateProtocol) BeforeSave(tx *gorm.DB) error {
	p.ProtocolID = strings.TrimSpace(strings.ToLower(p.ProtocolID))
	if p.ProtocolID == "" {
		return errors.New("connection_template_protocol: protocol_id is required")
	}

	p.DriverID = strings.TrimSpace(strings.ToLower(p.DriverID))
	if p.DriverID == "" {
		return errors.New("connection_template_protocol: driver_id is required")
	}

	p.Version = strings.TrimSpace(p.Version)
	if p.Version == "" {
		return errors.New("connection_template_protocol: version is required")
	}

	return nil
}
