package plugin

import (
	"context"
	"time"
)

// StorageScope selects a namespaced storage collection. Core enforces the
// platform boundaries. When Shared is false, core may default empty Plugin,
// ConnectionID, and OwnerID to the current request context. Shared namespaces
// opt out of those defaults so multiple plugins can intentionally share a
// namespace while still using OwnerID, Protocol, or other scope fields.
type StorageScope struct {
	Namespace    string
	Plugin       string
	Protocol     string
	ConnectionID string
	OwnerID      string
	Shared       bool
}

// StorageItem is one plugin storage record. Value is opaque to core; Metadata is
// indexed only by convention and intended for labels or lightweight filtering.
type StorageItem struct {
	Scope       StorageScope
	Key         string
	Value       []byte
	ContentType string
	Metadata    map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Storage is the generic persistence surface exposed to plugin route handlers.
// Implementations must scope access; plugins never receive raw database access.
type Storage interface {
	Get(ctx context.Context, scope StorageScope, key string) (StorageItem, error)
	Put(ctx context.Context, item StorageItem) (StorageItem, error)
	Delete(ctx context.Context, scope StorageScope, key string) error
	List(ctx context.Context, scope StorageScope, prefix string) ([]StorageItem, error)
}
