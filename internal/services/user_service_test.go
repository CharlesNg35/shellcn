package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestUserServiceSetRoles(t *testing.T) {
	db := openUserServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := NewUserService(db, auditSvc)
	require.NoError(t, err)

	ctx := context.Background()

	hashed, err := crypto.HashPassword("password123")
	require.NoError(t, err)

	user := &models.User{
		Username: "assign-role",
		Email:    "assign@example.com",
		Password: hashed,
	}
	require.NoError(t, db.Create(user).Error)

	role := &models.Role{
		BaseModel: models.BaseModel{ID: "role.viewer"},
		Name:      "Viewer",
	}
	require.NoError(t, db.Create(role).Error)

	updated, err := userSvc.SetRoles(ctx, user.ID, []string{role.ID})
	require.NoError(t, err)
	require.Len(t, updated.Roles, 1)
	require.Equal(t, role.ID, updated.Roles[0].ID)

	updated, err = userSvc.SetRoles(ctx, user.ID, nil)
	require.NoError(t, err)
	require.Len(t, updated.Roles, 0)
}

func openUserServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.Role{},
		&models.Permission{},
		&models.AuditLog{},
	))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
