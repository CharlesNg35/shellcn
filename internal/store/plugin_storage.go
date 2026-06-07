package store

import (
	"fmt"

	"github.com/charlesng35/shellcn/internal/models"
)

func validatePluginStoragePut(item *models.PluginStorageItem) error {
	if item == nil {
		return fmt.Errorf("%w: plugin storage item is required", models.ErrInvalidInput)
	}
	return item.Validate()
}

func pluginStorageKeyNeedsUniqueConnection(f PluginStorageFilter) bool {
	return f.Key != "" && f.ConnectionID == ""
}
