package models

import "time"

// UserInvite represents an invitation sent to a prospective user.
type UserInvite struct {
	BaseModel

	Email      string     `gorm:"not null;index" json:"email"`
	TokenHash  string     `gorm:"not null" json:"-"`
	InvitedBy  string     `gorm:"type:uuid" json:"invited_by"`
	ExpiresAt  time.Time  `gorm:"index" json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
}
