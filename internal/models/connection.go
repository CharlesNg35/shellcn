package models

import (
	"time"

	"gorm.io/datatypes"
)

// Connection represents a reusable remote access definition scoped to a team or user.
type Connection struct {
	BaseModel

	Name        string         `gorm:"not null;index" json:"name"`
	Description string         `json:"description"`
	ProtocolID  string         `gorm:"not null;index" json:"protocol_id"`
	TeamID      *string        `gorm:"type:uuid;index" json:"team_id"`
	OwnerUserID string         `gorm:"type:uuid;index" json:"owner_user_id"`
	FolderID    *string        `gorm:"type:uuid;index" json:"folder_id"`
	Metadata    datatypes.JSON `json:"metadata"`
	Settings    datatypes.JSON `json:"settings"`
	SecretID    *string        `gorm:"type:uuid" json:"secret_id"`
	LastUsedAt  *time.Time     `json:"last_used_at"`

	Targets    []ConnectionTarget     `gorm:"foreignKey:ConnectionID" json:"targets,omitempty"`
	Visibility []ConnectionVisibility `gorm:"foreignKey:ConnectionID" json:"visibility,omitempty"`
	Folder     *ConnectionFolder      `gorm:"foreignKey:FolderID" json:"folder,omitempty"`
}
