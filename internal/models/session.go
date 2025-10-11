package models

import "time"

type Session struct {
	BaseModel

	UserID       string     `gorm:"type:uuid;not null;index" json:"user_id"`
	User         *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	RefreshToken string     `gorm:"uniqueIndex;not null" json:"-"`
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	DeviceName   string     `json:"device_name"`
	ExpiresAt    time.Time  `gorm:"index" json:"expires_at"`
	LastUsedAt   time.Time  `json:"last_used_at"`
	RevokedAt    *time.Time `json:"revoked_at"`
}
