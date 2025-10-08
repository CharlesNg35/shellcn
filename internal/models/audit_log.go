package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	UserID    *string   `gorm:"type:uuid;index" json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Username  string    `json:"username"`
	Action    string    `gorm:"not null;index" json:"action"`
	Resource  string    `gorm:"index" json:"resource"`
	Result    string    `gorm:"not null" json:"result"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Metadata  string    `gorm:"type:json" json:"metadata"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
