package plugin

import (
	"context"
	"time"
)

type StorageScopeLevel string

const (
	StorageScopeConnection StorageScopeLevel = "connection"
	StorageScopeUser       StorageScopeLevel = "user"
)

func ConnectionStorage(namespace string) StorageScope {
	return StorageScope{Namespace: namespace, Level: StorageScopeConnection}
}

func UserStorage(namespace string) StorageScope {
	return StorageScope{Namespace: namespace, Level: StorageScopeUser}
}

// StorageScope filters a namespaced storage collection. Empty Level defaults to
// StorageScopeConnection. Core resolves and enforces plugin, connection, and
// user identity; plugins declare only the logical namespace and filter level.
type StorageScope struct {
	Namespace string
	Level     StorageScopeLevel
}

// StorageItem is one plugin storage record. Value is opaque to core; Metadata is
// intended for labels or lightweight local filtering.
type StorageItem struct {
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
	Put(ctx context.Context, namespace string, item StorageItem) (StorageItem, error)
	Delete(ctx context.Context, scope StorageScope, key string) error
	List(ctx context.Context, scope StorageScope) ([]StorageItem, error)
}
