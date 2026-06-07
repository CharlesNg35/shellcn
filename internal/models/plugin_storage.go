package models

import "time"

// PluginStorageItem is generic plugin-owned platform object storage.
// Core owns the scope columns so plugins never receive raw database access.
type PluginStorageItem struct {
	Namespace    string `gorm:"primaryKey"`
	Plugin       string `gorm:"primaryKey"`
	ConnectionID string `gorm:"primaryKey"`
	OwnerID      string `gorm:"primaryKey"`
	ItemKey      string `gorm:"primaryKey"`

	Value       []byte
	ContentType string
	Metadata    map[string]string `gorm:"serializer:json"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PluginStorageItem) TableName() string { return "plugin_storage_items" }
