package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestGetAndUpsertSystemSetting(t *testing.T) {
	db := openSystemSettingTestDB(t)

	value, err := GetSystemSetting(context.Background(), db, "missing")
	require.NoError(t, err)
	require.Equal(t, "", value)

	require.NoError(t, UpsertSystemSetting(context.Background(), db, "sample", "value1"))

	retrieved, err := GetSystemSetting(context.Background(), db, "sample")
	require.NoError(t, err)
	require.Equal(t, "value1", retrieved)

	require.NoError(t, UpsertSystemSetting(context.Background(), db, "sample", "value2"))

	retrieved, err = GetSystemSetting(context.Background(), db, "sample")
	require.NoError(t, err)
	require.Equal(t, "value2", retrieved)
}

func TestEnsureVaultEncryptionKey(t *testing.T) {
	db := openSystemSettingTestDB(t)

	require.NoError(t, EnsureVaultEncryptionKey(context.Background(), db, "initial"))

	value, err := GetSystemSetting(context.Background(), db, VaultEncryptionKeySetting)
	require.NoError(t, err)
	require.Equal(t, "initial", value)

	require.NoError(t, EnsureVaultEncryptionKey(context.Background(), db, "updated"))

	value, err = GetSystemSetting(context.Background(), db, VaultEncryptionKeySetting)
	require.NoError(t, err)
	require.Equal(t, "updated", value)
}

func openSystemSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
