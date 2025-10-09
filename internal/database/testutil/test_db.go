package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database"
)

// TestDBOption customises the behaviour of MustOpenTestDB.
type TestDBOption func(*testDBConfig)

type testDBConfig struct {
	autoMigrate bool
	seedData    bool
}

// WithAutoMigrate enables automatic schema migration after opening the test database.
func WithAutoMigrate() TestDBOption {
	return func(cfg *testDBConfig) {
		cfg.autoMigrate = true
	}
}

// WithSeedData ensures migrations are applied and default seed data inserted.
func WithSeedData() TestDBOption {
	return func(cfg *testDBConfig) {
		cfg.autoMigrate = true
		cfg.seedData = true
	}
}

// MustOpenTestDB opens an in-memory SQLite database for tests, applying optional migrations/seed data.
// The returned connection is automatically closed via t.Cleanup.
func MustOpenTestDB(t *testing.T, opts ...TestDBOption) *gorm.DB {
	t.Helper()

	cfg := testDBConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := database.Open(database.Config{Driver: "sqlite"})
	require.NoError(t, err)

	if cfg.seedData {
		require.NoError(t, database.AutoMigrateAndSeed(db))
	} else if cfg.autoMigrate {
		require.NoError(t, database.AutoMigrate(db))
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
