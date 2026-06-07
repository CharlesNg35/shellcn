package plugin

import (
	"context"
	"time"
)

// StorageScope selects a namespaced storage collection. Core resolves and
// enforces plugin, connection, and user identity; plugins declare only the
// logical namespace and whether data is connection-local or user-scoped across
// this plugin's connections.
type StorageScope struct {
	Namespace  string
	UserScoped bool
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
