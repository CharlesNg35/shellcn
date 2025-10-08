package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User describes platform users with relationships to organisations, teams, and roles.
type User struct {
	ID       string `gorm:"primaryKey;type:uuid" json:"id"`
	Username string `gorm:"uniqueIndex;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Avatar    string `json:"avatar"`

	IsRoot   bool `gorm:"default:false" json:"is_root"`
	IsActive bool `gorm:"default:true" json:"is_active"`

	MFAEnabled     bool          `gorm:"default:false" json:"mfa_enabled"`
	MFASecret      *MFASecret    `gorm:"foreignKey:UserID" json:"-"`
	OrganizationID *string       `gorm:"type:uuid" json:"organization_id"`
	Organization   *Organization `json:"organization,omitempty"`

	Teams    []Team    `gorm:"many2many:user_teams;" json:"teams,omitempty"`
	Roles    []Role    `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Sessions []Session `gorm:"foreignKey:UserID" json:"-"`

	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip"`

	FailedAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil    *time.Time `json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate ensures a UUID is present before persisting.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}
