package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestOrganizationServiceLifecycle(t *testing.T) {
	db := openOrganizationServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	orgSvc, err := NewOrganizationService(db, auditSvc)
	require.NoError(t, err)

	ctx := context.Background()

	org, err := orgSvc.Create(ctx, CreateOrganizationInput{
		Name:        "Acme Corp",
		Description: "Primary tenant",
		Settings: map[string]any{
			"timezone": "UTC",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, org.ID)

	retrieved, err := orgSvc.GetByID(ctx, org.ID)
	require.NoError(t, err)
	require.Equal(t, "Acme Corp", retrieved.Name)

	all, err := orgSvc.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)

	newDesc := "Updated description"
	updated, err := orgSvc.Update(ctx, org.ID, UpdateOrganizationInput{
		Description: &newDesc,
	})
	require.NoError(t, err)
	require.Equal(t, newDesc, updated.Description)

	require.NoError(t, orgSvc.Delete(ctx, org.ID))

	_, err = orgSvc.GetByID(ctx, org.ID)
	require.ErrorIs(t, err, ErrOrganizationNotFound)
}

func openOrganizationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.Organization{},
		&models.Team{},
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
