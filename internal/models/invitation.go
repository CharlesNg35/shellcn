package models

import "time"

// InvitationStatus tracks an invite through its lifecycle.
type InvitationStatus string

const (
	InvitePending  InvitationStatus = "pending"
	InviteAccepted InvitationStatus = "accepted"
	InviteRevoked  InvitationStatus = "revoked"
)

// Invitation is an emailed (or link-shared) offer to create an account with a
// preset role. Only the token hash is stored; the raw token lives in the link.
type Invitation struct {
	ID         string `gorm:"primaryKey"`
	Email      string `gorm:"index"`
	Role       Role
	TokenHash  string `gorm:"uniqueIndex"`
	Status     InvitationStatus
	InvitedBy  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AcceptedAt time.Time
}

func (Invitation) TableName() string { return "invitations" }

// InvitationSummary is the non-secret view returned to clients (no token).
type InvitationSummary struct {
	ID        string           `json:"id"`
	Email     string           `json:"email"`
	Role      Role             `json:"role"`
	Status    InvitationStatus `json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	ExpiresAt time.Time        `json:"expiresAt"`
}

// Summary projects an invitation, downgrading a lapsed pending invite to expired.
func (i Invitation) Summary() InvitationSummary {
	status := i.Status
	if status == InvitePending && time.Now().After(i.ExpiresAt) {
		status = InvitationStatus("expired")
	}
	return InvitationSummary{
		ID: i.ID, Email: i.Email, Role: i.Role, Status: status,
		CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
	}
}
