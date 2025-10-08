package models

import "time"

type Permission struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Module      string `gorm:"not null;index" json:"module"`
	Description string `json:"description"`
	DependsOn   string `gorm:"type:json" json:"depends_on"`

	Roles []Role `gorm:"many2many:role_permissions;" json:"roles,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}
