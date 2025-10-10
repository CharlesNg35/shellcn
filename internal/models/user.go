package models

import "time"

// User describes platform users with relationships to teams and roles.
type User struct {
	BaseModel

	Username string `gorm:"uniqueIndex;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Avatar    string `json:"avatar"`

	IsRoot   bool `gorm:"default:false" json:"is_root"`
	IsActive bool `gorm:"default:true" json:"is_active"`

	MFAEnabled bool       `gorm:"default:false" json:"mfa_enabled"`
	MFASecret  *MFASecret `gorm:"foreignKey:UserID" json:"-"`

	Teams    []Team    `gorm:"many2many:user_teams;" json:"teams,omitempty"`
	Roles    []Role    `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Sessions []Session `gorm:"foreignKey:UserID" json:"-"`

	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip"`

	FailedAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil    *time.Time `json:"-"`
}
