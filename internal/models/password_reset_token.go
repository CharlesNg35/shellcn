package models

import "time"

type PasswordResetToken struct {
	BaseModel

	UserID    string     `gorm:"type:uuid;not null;index" json:"user_id"`
	Token     string     `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
}
