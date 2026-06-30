package models

import "time"

// Access is the level a grant confers on a shared resource.
type Access string

const (
	AccessView       Access = "view"
	AccessManage     Access = "manage"
	AccessPrivileged Access = "privileged"
)

func ConnectionGrantAccesses() []Access {
	return []Access{AccessView, AccessManage, AccessPrivileged}
}

func CredentialGrantAccesses() []Access {
	return []Access{AccessView}
}

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

	AIMode             AIMode
	AIAllowDestructive bool
	AIAutoApprove      bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type AIMode string

// Empty AIMode preserves older rows and is treated as read_only.
const (
	AIModeDisabled  AIMode = "disabled"
	AIModeReadOnly  AIMode = "read_only"
	AIModeReadWrite AIMode = "read_write"
)

func (Connection) TableName() string { return "connections" }

// ConnectionFolder is a per-user sidebar grouping for visible connections.
type ConnectionFolder struct {
	ID        string `gorm:"primaryKey"`
	UserID    string `gorm:"index"`
	ParentID  string `gorm:"index"`
	Name      string
	Color     string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ConnectionFolder) TableName() string { return "connection_folders" }

// ConnectionPlacement stores a user's folder and ordering preference for one
// accessible connection. FolderID is empty for the root list.
type ConnectionPlacement struct {
	UserID       string `gorm:"primaryKey"`
	ConnectionID string `gorm:"primaryKey"`
	FolderID     string `gorm:"index"`
	SortOrder    int
	UpdatedAt    time.Time
}

func (ConnectionPlacement) TableName() string { return "connection_placements" }

// Grant is an explicit per-connection sharing grant to a subject (user).
type Grant struct {
	ID           string `gorm:"primaryKey"`
	ConnectionID string `gorm:"index;uniqueIndex:idx_grant_conn_subject"`
	SubjectID    string `gorm:"index;uniqueIndex:idx_grant_conn_subject"`
	Access       Access
	CreatedAt    time.Time
}

func (Grant) TableName() string { return "grants" }

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
