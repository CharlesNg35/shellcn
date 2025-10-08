package database

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"gorm.io/gorm"
)

func TestOpenSQLiteMemory(t *testing.T) {
	db := openTestDB(t)

	if err := db.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("expected health query to succeed: %v", err)
	}
}

func TestAutoMigrateAndSeedData(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrateAndSeed(db); err != nil {
		t.Fatalf("auto migrate and seed failed: %v", err)
	}

	var roleCount int64
	if err := db.Model(&models.Role{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount < 2 {
		t.Fatalf("expected at least 2 roles, got %d", roleCount)
	}

	var providerCount int64
	if err := db.Model(&models.AuthProvider{}).Count(&providerCount).Error; err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if providerCount < 2 {
		t.Fatalf("expected at least 2 auth providers, got %d", providerCount)
	}

	var permissionCount int64
	if err := db.Model(&models.Permission{}).Count(&permissionCount).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if permissionCount == 0 {
		t.Fatalf("expected at least 1 permission to be seeded")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := Open(Config{Driver: "sqlite"})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
