package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides shared fields for all persistent models.
type BaseModel struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` (Disable softDeletion)
}

// BeforeCreate ensures UUID identifiers are generated automatically.
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}
