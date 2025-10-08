package models

import "time"

type MFASecret struct {
	BaseModel

	UserID      string     `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Secret      string     `gorm:"not null" json:"-"`
	BackupCodes string     `gorm:"type:json" json:"-"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}
