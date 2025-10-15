package database

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/models"
)

type legacyConnection struct {
	models.BaseModel
	Name        string
	ProtocolID  string
	OwnerUserID string
	SecretID    *string
}

func TestAutoMigrateRenamesConnectionSecretID(t *testing.T) {
	db := openTestDB(t)

	require.NoError(t, db.AutoMigrate(&legacyConnection{}))

	migrator := db.Migrator()
	require.True(t, migrator.HasColumn(&legacyConnection{}, "secret_id"), "expected legacy column to exist")

	require.NoError(t, AutoMigrate(db))

	require.False(t, migrator.HasColumn(&models.Connection{}, "secret_id"), "expected secret_id column to be removed")
	require.True(t, migrator.HasColumn(&models.Connection{}, "identity_id"), "expected identity_id column to exist")
}

func TestAutoMigrateCreatesVaultTables(t *testing.T) {
	db := openTestDB(t)

	require.NoError(t, AutoMigrate(db))

	migrator := db.Migrator()
	tables := []interface{}{
		&models.Identity{},
		&models.IdentityShare{},
		&models.CredentialTemplate{},
		&models.CredentialVersion{},
		&models.VaultKeyMetadata{},
	}

	for _, table := range tables {
		require.True(t, migrator.HasTable(table), "expected table for %T to exist", table)
	}
}

func TestAutoMigrateCreatesSessionTables(t *testing.T) {
	db := openTestDB(t)

	require.NoError(t, AutoMigrate(db))

	migrator := db.Migrator()
	tables := []interface{}{
		&models.ConnectionSession{},
		&models.ConnectionSessionParticipant{},
		&models.ConnectionSessionMessage{},
		&models.ConnectionSessionRecord{},
	}

	for _, table := range tables {
		require.True(t, migrator.HasTable(table), "expected table for %T to exist", table)
	}
}
