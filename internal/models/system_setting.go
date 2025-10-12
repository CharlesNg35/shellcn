package models

import "time"

// SystemSetting persists installation-wide values that should survive restarts.
type SystemSetting struct {
	Key       string    `gorm:"primaryKey"`
	Value     string    `gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
