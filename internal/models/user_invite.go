package models

import "time"

// UserInvite represents an invitation sent to a prospective user.
type UserInvite struct {
	BaseModel

	Email      string     `gorm:"not null;index" json:"email"`
	TokenHash  string     `gorm:"not null" json:"-"`
	InvitedBy  string     `gorm:"type:uuid" json:"invited_by"`
	TeamID     *string    `gorm:"type:uuid;index" json:"team_id,omitempty"`
	ExpiresAt  time.Time  `gorm:"index" json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`

	Team *Team `gorm:"constraint:OnDelete:SET NULL" json:"team,omitempty"`
}
