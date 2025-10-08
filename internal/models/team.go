package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Team struct {
	ID          string `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`

	OrganizationID string        `gorm:"type:uuid;not null" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Users          []User        `gorm:"many2many:user_teams;" json:"users,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}
