package models

import (
	"time"
)

// CacheEntry represents a cached value stored in the database fallback.
type CacheEntry struct {
	Key       string    `gorm:"primaryKey;size:256"`
	Value     []byte    `gorm:"type:blob"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
