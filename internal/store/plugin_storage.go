package store

import (
	"fmt"

	"github.com/charlesng35/shellcn/internal/models"
)

type pluginStorageFilterMode uint8

const (
	pluginStorageFilterRead pluginStorageFilterMode = iota
	pluginStorageFilterListOrDelete
)

func validatePluginStoragePut(item *models.PluginStorageItem) error {
	if item == nil {
		return fmt.Errorf("%w: plugin storage item is required", models.ErrInvalidInput)
	}
	return item.Validate()
}

func validatePluginStorageFilter(f PluginStorageFilter, mode pluginStorageFilterMode) error {
	switch {
	case f.Collection == "":
		return fmt.Errorf("%w: plugin storage collection is required", models.ErrInvalidInput)
	case f.Plugin == "":
		return fmt.Errorf("%w: plugin storage plugin is required", models.ErrInvalidInput)
	case f.OwnerID == "":
		return fmt.Errorf("%w: plugin storage owner_id is required", models.ErrInvalidInput)
	case mode == pluginStorageFilterRead && f.Key == "":
		return fmt.Errorf("%w: plugin storage key is required", models.ErrInvalidInput)
	case f.Limit < 0:
		return fmt.Errorf("%w: plugin storage limit cannot be negative", models.ErrInvalidInput)
	case f.Offset < 0:
		return fmt.Errorf("%w: plugin storage offset cannot be negative", models.ErrInvalidInput)
	default:
		return nil
	}
}

func pluginStorageKeyNeedsUniqueConnection(f PluginStorageFilter) bool {
	return f.Key != "" && f.ConnectionID == ""
}
