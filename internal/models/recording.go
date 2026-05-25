package models

import "time"

// RecordingStatus is the lifecycle state of a session recording.
type RecordingStatus string

const (
	RecordingPending   RecordingStatus = "pending"   // metadata created, not yet writing
	RecordingActive    RecordingStatus = "active"    // capturing
	RecordingFinalized RecordingStatus = "finalized" // complete and playable
	RecordingFailed    RecordingStatus = "failed"    // capture errored
	RecordingDiscarded RecordingStatus = "discarded" // blob removed (retention/abort)
)

// Recording is the control-plane metadata for one captured session. The bytes
// live in a blob store keyed by StorageKey; this row is the queryable index and
// never holds secret material.
type Recording struct {
	ID             string `gorm:"primaryKey"`
	UserID         string `gorm:"index"`
	Username       string
	ConnectionID   string `gorm:"index"`
	ConnectionName string
	Protocol       string `gorm:"index"`
	RouteID        string
	StreamID       string
	Class          string `gorm:"index"` // terminal | desktop
	Format         string // asciicast_v2 | webm_canvas | ...
	Authoritative  bool
	Status         RecordingStatus `gorm:"index"`
	Title          string
	StartedAt      time.Time
	EndedAt        *time.Time
	DurationMS     int64
	Size           int64
	Checksum       string // sha256 hex of the finalized blob
	StorageKey     string
	Error          string
	ExpiresAt      *time.Time `gorm:"index"` // nil = retained indefinitely
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Recording) TableName() string { return "recordings" }
