package models

import "gorm.io/datatypes"

// ConnectionTarget lists endpoints associated with a connection.
type ConnectionTarget struct {
	BaseModel

	ConnectionID string         `gorm:"type:uuid;index" json:"connection_id"`
	Host         string         `gorm:"not null" json:"host"`
	Port         int            `json:"port"`
	Labels       datatypes.JSON `json:"labels"`
	Ordering     int            `gorm:"default:0" json:"ordering"`
}
