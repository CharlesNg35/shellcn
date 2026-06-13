package store

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestGormLoggerSuppressesRecordNotFound(t *testing.T) {
	var buf bytes.Buffer
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: gormLoggerWithWriter(log.New(&buf, "", 0), logger.Warn),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ClusterOwner{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	var owner models.ClusterOwner
	err = db.First(&owner, "owner_key = ?", "missing").Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("first err = %v, want ErrRecordNotFound", err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("record-not-found log output = %q, want empty", got)
	}
}
