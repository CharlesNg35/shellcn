package models

// ConnectionVisibility stores ACL style visibility for connections.
type ConnectionVisibility struct {
	BaseModel

	ConnectionID    string  `gorm:"type:uuid;index" json:"connection_id"`
	TeamID          *string `gorm:"type:uuid;index" json:"team_id"`
	UserID          *string `gorm:"type:uuid;index" json:"user_id"`
	PermissionScope string  `gorm:"type:varchar(32);index" json:"permission_scope"`
}
