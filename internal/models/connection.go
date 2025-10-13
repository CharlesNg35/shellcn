package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
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
	IdentityID  *string        `gorm:"type:uuid" json:"identity_id"`
	LastUsedAt  *time.Time     `json:"last_used_at"`

	Targets        []ConnectionTarget   `gorm:"foreignKey:ConnectionID" json:"targets,omitempty"`
	ResourceGrants []ResourcePermission `gorm:"polymorphic:Resource;polymorphicValue:connection" json:"resource_grants,omitempty"`
	Folder         *ConnectionFolder    `gorm:"foreignKey:FolderID" json:"folder,omitempty"`
}

// This ensures orphaned resource_permissions and connection_targets are cleaned up.
func (c *Connection) BeforeDelete(tx *gorm.DB) error {
	// Delete all resource permissions associated with this connection
	if err := tx.Where("resource_type = ? AND resource_id = ?", "connection", c.ID).
		Delete(&ResourcePermission{}).Error; err != nil {
		return err
	}

	// Delete all connection targets associated with this connection
	if err := tx.Where("connection_id = ?", c.ID).
		Delete(&ConnectionTarget{}).Error; err != nil {
		return err
	}

	return nil
}
