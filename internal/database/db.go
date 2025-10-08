package database

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Config contains database connection options.
type Config struct {
	Driver string
	Path   string // SQLite database path when Driver == sqlite
	DSN    string // Optional DSN override
}

// Open initialises a gorm.DB using the provided configuration.
func Open(cfg Config) (*gorm.DB, error) {
	driver := strings.ToLower(cfg.Driver)
	if driver == "" {
		driver = "sqlite"
	}

	switch driver {
	case "sqlite":
		return openSQLite(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

// AutoMigrateAndSeed convenience helper used during application start-up.
func AutoMigrateAndSeed(db *gorm.DB) error {
	if db == nil {
		return errors.New("nil database handle")
	}

	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	if err := SeedData(db); err != nil {
		return fmt.Errorf("seed data: %w", err)
	}

	return nil
}
