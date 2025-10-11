package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSQLite(cfg Config) (*gorm.DB, error) {
	dsn := cfg.DSN

	if dsn == "" {
		path := strings.TrimSpace(cfg.Path)
		switch {
		case path == "", strings.EqualFold(path, ":memory:"):
			dsn = "file::memory:?cache=shared&_foreign_keys=1"
		default:
			if err := ensureDir(path); err != nil {
				return nil, err
			}
			dsn = fmt.Sprintf("file:%s?_foreign_keys=1&_journal_mode=WAL", filepath.ToSlash(path))
		}
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, err
	}

	if err := enableForeignKeys(db); err != nil {
		return nil, err
	}

	return db, nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func enableForeignKeys(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil && err != sql.ErrConnDone {
		return err
	}
	return nil
}
