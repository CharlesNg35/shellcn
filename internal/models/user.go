package models

import (
	"slices"
	"time"
)

// Role is a coarse platform role; fine-grained access is layered via grants.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// User is an authenticated platform principal — used for authz and audit. It is
// also the GORM model; PasswordHash never serializes to clients (json:"-") and
// is cleared by the store before a User leaves the persistence layer.
type User struct {
	ID           string `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	Email        string
	DisplayName  string
	Roles        []Role `gorm:"serializer:json"`
	PasswordHash string `gorm:"column:password_hash" json:"-"`
	Disabled     bool
	// Protected marks the root admin, which can never be deleted.
	Protected bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (User) TableName() string { return "users" }

// HasRole reports whether the user holds the given role.
func (u User) HasRole(r Role) bool {
	return slices.Contains(u.Roles, r)
}
