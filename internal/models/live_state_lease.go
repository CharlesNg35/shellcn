package models

import "time"

type LiveStateLease struct {
	Key          string `gorm:"primaryKey;column:lease_key"`
	InstanceID   string `gorm:"index;not null"`
	InternalURL  string
	InternalURLs string
	LeaseID      string    `gorm:"index;not null"`
	ExpiresAt    time.Time `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (LiveStateLease) TableName() string { return "live_state_leases" }
