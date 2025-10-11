package models

import "time"

// EmailVerification stores verification tokens for local registrations.
type EmailVerification struct {
	BaseModel

	UserID     string     `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash  string     `gorm:"not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"index" json:"expires_at"`
	VerifiedAt *time.Time `json:"verified_at"`
}
