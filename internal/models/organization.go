package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Organization struct {
	ID          string `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Settings    string `gorm:"type:json" json:"settings"`

	Users []User `gorm:"foreignKey:OrganizationID" json:"users,omitempty"`
	Teams []Team `gorm:"foreignKey:OrganizationID" json:"teams,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	return nil
}
