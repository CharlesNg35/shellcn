package models

import (
	"time"

	"gorm.io/datatypes"
)

// ResourcePermission stores fine-grained grants tied to specific resources.
type ResourcePermission struct {
	BaseModel

	ResourceID    string         `gorm:"type:uuid;not null;index:idx_resource_principal,priority:1" json:"resource_id"`
	ResourceType  string         `gorm:"type:varchar(64);not null;index" json:"resource_type"`
	PrincipalID   string         `gorm:"type:uuid;not null;index:idx_resource_principal,priority:3" json:"principal_id"`
	PrincipalType string         `gorm:"type:varchar(16);not null;index:idx_resource_principal,priority:2" json:"principal_type"`
	PermissionID  string         `gorm:"type:varchar(128);not null;index" json:"permission_id"`
	GrantedByID   *string        `gorm:"type:uuid;index" json:"granted_by_id"`
	ExpiresAt     *time.Time     `json:"expires_at"`
	Metadata      datatypes.JSON `json:"metadata"`
}

// TableName overrides the default table name for GORM.
func (ResourcePermission) TableName() string {
	return "resource_permissions"
}
