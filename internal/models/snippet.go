package models

import (
	"strings"
)

// Snippet represents a reusable command snippet that can be executed within SSH sessions.
type Snippet struct {
	BaseModel

	Name         string  `gorm:"type:varchar(120);not null" json:"name"`
	Description  string  `gorm:"type:text" json:"description,omitempty"`
	Command      string  `gorm:"type:text;not null" json:"command"`
	Scope        string  `gorm:"type:varchar(20);not null;index" json:"scope"`
	OwnerUserID  *string `gorm:"type:uuid;index" json:"owner_user_id,omitempty"`
	ConnectionID *string `gorm:"type:uuid;index" json:"connection_id,omitempty"`
}

// Normalise ensures the scope value is lower-cased.
func (s *Snippet) Normalise() {
	s.Scope = strings.ToLower(strings.TrimSpace(s.Scope))
}
