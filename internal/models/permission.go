package models

import "gorm.io/datatypes"

type Permission struct {
	BaseModel

	Module      string         `gorm:"not null;index" json:"module"`
	Description string         `json:"description"`
	DependsOn   datatypes.JSON `json:"depends_on"`
	Implies     datatypes.JSON `json:"implies"`

	Roles []Role `gorm:"many2many:role_permissions;" json:"roles,omitempty"`
}
