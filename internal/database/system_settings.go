package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

const VaultEncryptionKeySetting = "vault.encryption_key"

// GetSystemSetting retrieves a system setting by key. Returns an empty string when not found.
func GetSystemSetting(ctx context.Context, db *gorm.DB, key string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("system settings: db is nil")
	}

	var setting models.SystemSetting
	err := db.WithContext(ctx).Take(&setting, "key = ?", key).Error
	if err == nil {
		return setting.Value, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if strings.Contains(err.Error(), "no such table") {
		return "", nil
	}
	return "", fmt.Errorf("system settings: get %q: %w", key, err)
}

// UpsertSystemSetting stores or updates a system setting value.
func UpsertSystemSetting(ctx context.Context, db *gorm.DB, key, value string) error {
	if db == nil {
		return fmt.Errorf("system settings: db is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("system settings: key is required")
	}

	record := models.SystemSetting{
		Key:   key,
		Value: value,
	}

	if err := db.WithContext(ctx).
		Where("key = ?", key).
		Assign(map[string]any{"value": value}).
		FirstOrCreate(&record).Error; err != nil {
		return fmt.Errorf("system settings: upsert %q: %w", key, err)
	}

	return nil
}

// EnsureVaultEncryptionKey stores the supplied vault encryption key if no key exists yet,
// and updates the stored value when it differs from the new key.
func EnsureVaultEncryptionKey(ctx context.Context, db *gorm.DB, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("system settings: vault key is empty")
	}

	current, err := GetSystemSetting(ctx, db, VaultEncryptionKeySetting)
	if err != nil {
		return err
	}

	if strings.TrimSpace(current) == key {
		return nil
	}

	return UpsertSystemSetting(ctx, db, VaultEncryptionKeySetting, key)
}
