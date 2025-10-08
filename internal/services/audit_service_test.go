package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestAuditServiceLogListAndExport(t *testing.T) {
	db := openAuditServiceTestDB(t)
	svc, err := NewAuditService(db)
	require.NoError(t, err)

	hashed, err := crypto.HashPassword("secret123!")
	require.NoError(t, err)

	user := models.User{
		Username: "auditor",
		Email:    "auditor@example.com",
		Password: hashed,
	}
	require.NoError(t, db.Create(&user).Error)

	ctx := context.Background()
	err = svc.Log(ctx, AuditEntry{
		UserID:   &user.ID,
		Username: "auditor",
		Action:   "user.create",
		Resource: "users",
		Result:   "success",
		Metadata: map[string]any{"email": user.Email},
	})
	require.NoError(t, err)

	logs, total, err := svc.List(ctx, AuditListOptions{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, "user.create", logs[0].Action)
	require.NotNil(t, logs[0].User)
	require.Equal(t, user.ID, logs[0].User.ID)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].Metadata), &metadata))
	require.Equal(t, user.Email, metadata["email"])

	exported, err := svc.Export(ctx, AuditFilters{Result: "success"})
	require.NoError(t, err)
	require.Len(t, exported, 1)
}

func TestAuditServiceCleanupOlderThan(t *testing.T) {
	db := openAuditServiceTestDB(t)
	svc, err := NewAuditService(db)
	require.NoError(t, err)

	oldLog := models.AuditLog{
		BaseModel: models.BaseModel{
			CreatedAt: time.Now().AddDate(0, 0, -10),
		},
		Action:   "old.action",
		Result:   "success",
		Metadata: "{}",
	}
	require.NoError(t, db.Create(&oldLog).Error)

	ctx := context.Background()
	rows, err := svc.CleanupOlderThan(ctx, 5)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
}

func openAuditServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.AuditLog{},
	))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
