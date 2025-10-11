package models

import "gorm.io/datatypes"

// ConnectionFolder organizes connections into hierarchical groups.
type ConnectionFolder struct {
	BaseModel

	Name        string         `gorm:"not null" json:"name"`
	Slug        string         `gorm:"index" json:"slug"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Color       string         `json:"color"`
	Ordering    int            `gorm:"default:0" json:"ordering"`
	ParentID    *string        `gorm:"type:uuid;index" json:"parent_id"`
	TeamID      *string        `gorm:"type:uuid;index" json:"team_id"`
	OwnerUserID string         `gorm:"type:uuid;index" json:"owner_user_id"`
	Metadata    datatypes.JSON `json:"metadata"`

	Children    []ConnectionFolder `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Connections []Connection       `gorm:"foreignKey:FolderID" json:"connections,omitempty"`
}
