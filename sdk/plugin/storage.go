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

func ConnectionStorage(collection string) StorageScope {
	return StorageScope{Collection: collection, Level: StorageScopeConnection}
}

func UserStorage(collection string) StorageScope {
	return StorageScope{Collection: collection, Level: StorageScopeUser}
}

// StorageScope filters a storage collection. Empty Level defaults to
// StorageScopeConnection. Core resolves and enforces plugin, connection, and
// user identity; plugins declare only the logical collection and filter level.
type StorageScope struct {
	Collection string
	Level      StorageScopeLevel
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
	Put(ctx context.Context, collection string, item StorageItem) (StorageItem, error)
	Delete(ctx context.Context, scope StorageScope, key string) error
	List(ctx context.Context, scope StorageScope) ([]StorageItem, error)
}
