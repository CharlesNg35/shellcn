package models

import (
	"time"

	"gorm.io/datatypes"
)

// ConnectionSession represents a persisted record of a protocol session lifecycle.
type ConnectionSession struct {
	BaseModel

	ConnectionID    string                         `gorm:"type:uuid;not null;index" json:"connection_id"`
	ProtocolID      string                         `gorm:"not null;index" json:"protocol_id"`
	OwnerUserID     string                         `gorm:"type:uuid;not null;index" json:"owner_user_id"`
	TeamID          *string                        `gorm:"type:uuid;index" json:"team_id,omitempty"`
	Status          string                         `gorm:"type:varchar(32);not null;index" json:"status"`
	StartedAt       time.Time                      `gorm:"not null;index" json:"started_at"`
	ClosedAt        *time.Time                     `gorm:"index" json:"closed_at,omitempty"`
	LastHeartbeatAt time.Time                      `gorm:"index" json:"last_heartbeat_at"`
	Metadata        datatypes.JSON                 `gorm:"type:json" json:"metadata,omitempty"`
	Notes           string                         `gorm:"type:text" json:"notes,omitempty"`
	Connection      *Connection                    `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
	Owner           *User                          `gorm:"foreignKey:OwnerUserID" json:"owner,omitempty"`
	Participants    []ConnectionSessionParticipant `gorm:"foreignKey:SessionID" json:"participants,omitempty"`
	Messages        []ConnectionSessionMessage     `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
	Records         []ConnectionSessionRecord      `gorm:"foreignKey:SessionID" json:"records,omitempty"`
}

// ConnectionSessionParticipant stores per-user participation metadata for active sessions.
type ConnectionSessionParticipant struct {
	SessionID            string         `gorm:"type:uuid;primaryKey" json:"session_id"`
	UserID               string         `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role                 string         `gorm:"type:varchar(20);not null;index" json:"role"`
	AccessMode           string         `gorm:"type:varchar(20);not null;index" json:"access_mode"`
	GrantedByUserID      *string        `gorm:"type:uuid" json:"granted_by_user_id,omitempty"`
	JoinedAt             time.Time      `gorm:"not null;index" json:"joined_at"`
	LeftAt               *time.Time     `gorm:"index" json:"left_at,omitempty"`
	ConsentedToRecording bool           `gorm:"not null;default:false" json:"consented_to_recording"`
	Metadata             datatypes.JSON `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt            time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	User                 *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// ConnectionSessionMessage captures chat entries exchanged during a session.
type ConnectionSessionMessage struct {
	BaseModel

	SessionID string         `gorm:"type:uuid;not null;index" json:"session_id"`
	AuthorID  string         `gorm:"type:uuid;not null;index" json:"author_id"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	Metadata  datatypes.JSON `gorm:"type:json" json:"metadata,omitempty"`

	Session *ConnectionSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Author  *User              `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

// ConnectionSessionRecord maps recordings produced for a session.
type ConnectionSessionRecord struct {
	BaseModel

	SessionID       string         `gorm:"type:uuid;not null;index" json:"session_id"`
	StorageKind     string         `gorm:"type:varchar(16);not null;index" json:"storage_kind"`
	StoragePath     string         `gorm:"not null" json:"storage_path"`
	SizeBytes       int64          `gorm:"not null;default:0" json:"size_bytes"`
	DurationSeconds int64          `gorm:"not null;default:0" json:"duration_seconds"`
	Checksum        string         `gorm:"type:varchar(128)" json:"checksum,omitempty"`
	CreatedByUserID string         `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	RetentionUntil  *time.Time     `gorm:"index" json:"retention_until,omitempty"`
	Protected       bool           `gorm:"not null;default:false" json:"protected"`
	Metadata        datatypes.JSON `gorm:"type:json" json:"metadata,omitempty"`

	Session *ConnectionSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Creator *User              `gorm:"foreignKey:CreatedByUserID" json:"creator,omitempty"`
}
