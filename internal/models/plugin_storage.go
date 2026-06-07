package models

import (
	"fmt"
	"time"
)

// PluginStorageItem is generic plugin-owned platform object storage.
// Core owns the scope columns so plugins never receive raw database access.
type PluginStorageItem struct {
	Namespace    string `gorm:"primaryKey;not null;check:plugin_storage_namespace_required,namespace <> ''"`
	Plugin       string `gorm:"primaryKey;not null;check:plugin_storage_plugin_required,plugin <> ''"`
	ConnectionID string `gorm:"primaryKey;not null;check:plugin_storage_connection_required,connection_id <> ''"`
	OwnerID      string `gorm:"primaryKey;not null;check:plugin_storage_owner_required,owner_id <> ''"`
	ItemKey      string `gorm:"primaryKey;not null;check:plugin_storage_key_required,item_key <> ''"`

	Value       []byte
	ContentType string
	Metadata    map[string]string `gorm:"serializer:json"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PluginStorageItem) TableName() string { return "plugin_storage_items" }

func (i PluginStorageItem) Validate() error {
	switch {
	case i.Namespace == "":
		return fmt.Errorf("%w: plugin storage namespace is required", ErrInvalidInput)
	case i.Plugin == "":
		return fmt.Errorf("%w: plugin storage plugin is required", ErrInvalidInput)
	case i.ConnectionID == "":
		return fmt.Errorf("%w: plugin storage connection_id is required", ErrInvalidInput)
	case i.OwnerID == "":
		return fmt.Errorf("%w: plugin storage owner_id is required", ErrInvalidInput)
	case i.ItemKey == "":
		return fmt.Errorf("%w: plugin storage key is required", ErrInvalidInput)
	default:
		return nil
	}
}
