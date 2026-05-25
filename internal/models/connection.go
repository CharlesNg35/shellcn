package models

import "time"

// Access is the level a grant confers on a shared resource.
type Access string

const (
	AccessUse    Access = "use"    // connect through / read non-secret metadata
	AccessManage Access = "manage" // edit/share/delete
)

// Connection is stored config describing how to reach one target. It is owned by
// a user, optionally shared, and may carry inline encrypted secrets or reference
// reusable credentials.
type Connection struct {
	ID       string `gorm:"primaryKey"`
	Name     string
	Protocol string `gorm:"index"`
	OwnerID  string `gorm:"index"`
	// Transport is "direct" or "agent".
	Transport string
	Shared    bool

	// Config holds non-secret connection fields (host, port, …).
	Config map[string]any `gorm:"serializer:json"`
	// Secrets holds ciphertext for inline Secret==true fields, keyed by field key.
	// The store only ever sees ciphertext; encryption happens in the service layer.
	Secrets map[string][]byte `gorm:"serializer:json"`

	// Recording is the per-class recording policy (class -> disabled|manual|auto).
	// Absent/empty means recording is off, which is the default.
	Recording map[string]string `gorm:"serializer:json"`
	// RetentionDays caps how long this connection's recordings are kept; 0 = keep.
	RetentionDays int

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Connection) TableName() string { return "connections" }

// Grant is an explicit per-connection sharing grant to a subject (user).
type Grant struct {
	ID           string `gorm:"primaryKey"`
	ConnectionID string `gorm:"index;uniqueIndex:idx_grant_conn_subject"`
	SubjectID    string `gorm:"index;uniqueIndex:idx_grant_conn_subject"`
	Access       Access
	CreatedAt    time.Time
}

func (Grant) TableName() string { return "grants" }

// Snippet is a saved command/query body scoped to a protocol and owner.
type Snippet struct {
	ID        string `gorm:"primaryKey"`
	OwnerID   string `gorm:"index"`
	Protocol  string `gorm:"index"`
	Name      string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Snippet) TableName() string { return "snippets" }

// AgentEnrollmentStatus tracks the lifecycle of an agent enrollment.
type AgentEnrollmentStatus string

const (
	EnrollmentPending AgentEnrollmentStatus = "pending"
	EnrollmentOnline  AgentEnrollmentStatus = "online"
	EnrollmentOffline AgentEnrollmentStatus = "offline"
	EnrollmentExpired AgentEnrollmentStatus = "expired"
	EnrollmentRevoked AgentEnrollmentStatus = "revoked"
)

// AgentEnrollment binds an agent install token to one connection + proxy target.
type AgentEnrollment struct {
	ID           string `gorm:"primaryKey"`
	ConnectionID string `gorm:"index"`
	TokenHash    string `gorm:"uniqueIndex"` // never the raw token
	Status       AgentEnrollmentStatus
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (AgentEnrollment) TableName() string { return "agent_enrollments" }

// Preference is a per-user key/value (e.g. a connection's layout override).
type Preference struct {
	UserID    string `gorm:"primaryKey"`
	Key       string `gorm:"primaryKey;column:pref_key"`
	Value     string // JSON-encoded value
	UpdatedAt time.Time
}

func (Preference) TableName() string { return "preferences" }
