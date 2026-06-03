package models

import "time"

// AuditResult is the outcome recorded for an audited operation.
type AuditResult string

const (
	AuditAllowed AuditResult = "allowed"
	AuditDenied  AuditResult = "denied"
	AuditError   AuditResult = "error"
)

// AuditEntry is one append-only audit record. Params are redacted before write.
type AuditEntry struct {
	ID           string    `gorm:"primaryKey"`
	Time         time.Time `gorm:"index"`
	UserID       string    `gorm:"index"`
	Username     string
	Event        string `gorm:"index"` // route AuditEvent, e.g. "vm.snapshot.list"
	ConnectionID string `gorm:"index"`
	RouteID      string
	Risk         string
	Result       AuditResult
	Params       map[string]string `gorm:"serializer:json"` // secrets already redacted
	Error        string
	RemoteAddr   string
	// Source distinguishes how the operation was initiated: "http" for a direct
	// request, "ai" for an agent tool call. TurnID correlates AI-initiated calls
	// to their conversation/turn.
	Source string `gorm:"index"`
	TurnID string
}

func (AuditEntry) TableName() string { return "audit_entries" }
