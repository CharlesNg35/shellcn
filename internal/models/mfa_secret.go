package models

import (
	"time"

	"gorm.io/datatypes"
)

type MFASecret struct {
	BaseModel

	UserID      string         `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Secret      string         `gorm:"not null" json:"-"`
	BackupCodes datatypes.JSON `json:"-"`
	LastUsedAt  *time.Time     `json:"last_used_at"`
}
