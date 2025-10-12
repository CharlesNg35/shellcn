package models

import "gorm.io/datatypes"

// ConnectionProtocol stores metadata about registered protocols and their enablement state.
type ConnectionProtocol struct {
	BaseModel

	Name         string         `gorm:"not null" json:"name"`
	ProtocolID   string         `gorm:"not null;uniqueIndex" json:"protocol_id"`
	DriverID     string         `gorm:"not null" json:"driver_id"`
	Module       string         `gorm:"not null" json:"module"` // Maps to config.protocols namespace for enablement checks
	Icon         string         `json:"icon"`
	Category     string         `json:"category"`
	Description  string         `json:"description"`
	DefaultPort  int            `json:"default_port"`
	SortOrder    int            `gorm:"default:0" json:"sort_order"`
	Features     datatypes.JSON `json:"features"`
	Capabilities datatypes.JSON `json:"capabilities"`

	DriverEnabled bool `gorm:"default:false" json:"driver_enabled"`
	ConfigEnabled bool `gorm:"default:false" json:"config_enabled"`
}
