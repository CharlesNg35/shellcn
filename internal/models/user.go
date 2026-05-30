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
	// SessionVersion invalidates existing browser sessions when sensitive
	// account state changes.
	SessionVersion int
	Disabled       bool
	// Protected marks the root admin, which can never be deleted.
	Protected bool

	// Two-factor authentication (TOTP). TOTPSecret holds the encrypted shared
	// secret and never serializes to clients. A non-empty secret with
	// TOTPEnabled=false is an in-progress enrollment awaiting code confirmation.
	TOTPSecret         []byte   `gorm:"column:totp_secret" json:"-"`
	TOTPEnabled        bool     `gorm:"column:totp_enabled"`
	RecoveryCodeHashes []string `gorm:"serializer:json" json:"-"`
	// MFARemindedAt is when the user was last nudged to enable 2FA (nil = never).
	MFARemindedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TOTPPending reports an enrollment that has a secret but isn't confirmed yet.
func (u User) TOTPPending() bool {
	return len(u.TOTPSecret) > 0 && !u.TOTPEnabled
}

func (User) TableName() string { return "users" }

// HasRole reports whether the user holds the given role.
func (u User) HasRole(r Role) bool {
	return slices.Contains(u.Roles, r)
}
