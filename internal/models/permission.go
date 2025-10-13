package models

import "gorm.io/datatypes"

type Permission struct {
	BaseModel

	Module       string         `gorm:"not null;index" json:"module"`
	DisplayName  string         `json:"display_name"`
	Category     string         `json:"category"`
	Description  string         `json:"description"`
	DefaultScope string         `json:"default_scope"`
	Metadata     datatypes.JSON `json:"metadata"`
	DependsOn    datatypes.JSON `json:"depends_on"`
	Implies      datatypes.JSON `json:"implies"`

	Roles []Role `gorm:"many2many:role_permissions;" json:"roles,omitempty"`
}
