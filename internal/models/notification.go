package models

import (
	"time"

	"gorm.io/datatypes"
)

// Notification represents an in-app notification for a user.
type Notification struct {
	BaseModel

	UserID    string         `gorm:"type:uuid;index" json:"user_id"`
	Type      string         `gorm:"type:varchar(64);not null" json:"type"`
	Title     string         `gorm:"type:varchar(255);not null" json:"title"`
	Message   string         `gorm:"type:text" json:"message"`
	Severity  string         `gorm:"type:varchar(32);default:'info'" json:"severity"`
	ActionURL string         `gorm:"type:text" json:"action_url"`
	Metadata  datatypes.JSON `json:"metadata"`

	IsRead bool       `gorm:"default:false;index" json:"is_read"`
	ReadAt *time.Time `json:"read_at"`
}
