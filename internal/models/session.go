package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Session struct {
	ID           string     `gorm:"primaryKey;type:uuid" json:"id"`
	UserID       string     `gorm:"type:uuid;not null;index" json:"user_id"`
	User         *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	RefreshToken string     `gorm:"uniqueIndex;not null" json:"-"`
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	DeviceName   string     `json:"device_name"`
	ExpiresAt    time.Time  `gorm:"index" json:"expires_at"`
	LastUsedAt   time.Time  `json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
