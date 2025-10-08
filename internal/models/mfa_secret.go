package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MFASecret struct {
	ID          string     `gorm:"primaryKey;type:uuid" json:"id"`
	UserID      string     `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Secret      string     `gorm:"not null" json:"-"`
	BackupCodes string     `gorm:"type:json" json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}

func (m *MFASecret) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}
