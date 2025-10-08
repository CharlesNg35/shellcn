package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func TestPermissionService_CreateAndListRole(t *testing.T) {
	db, svc := setupPermissionServiceTest(t)

	role, err := svc.CreateRole(context.Background(), CreateRoleInput{
		Name:        "Custom",
		Description: "Custom role",
	})
	require.NoError(t, err)
	require.NotEmpty(t, role.ID)

	roles, err := svc.ListRoles(context.Background())
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, "Custom", roles[0].Name)

	var stored models.Role
	require.NoError(t, db.First(&stored, "id = ?", role.ID).Error)
	require.Equal(t, "Custom", stored.Name)
}

func TestPermissionService_DeleteSystemRole(t *testing.T) {
	_, svc := setupPermissionServiceTest(t)

	role, err := svc.CreateRole(context.Background(), CreateRoleInput{
		Name:     "System",
		IsSystem: true,
	})
	require.NoError(t, err)

	err = svc.DeleteRole(context.Background(), role.ID)
	require.ErrorIs(t, err, ErrSystemRoleImmutable)
}

func TestPermissionService_SetRolePermissionsIncludesDependencies(t *testing.T) {
	db, svc := setupPermissionServiceTest(t)

	role, err := svc.CreateRole(context.Background(), CreateRoleInput{
		Name: "Permission Tester",
	})
	require.NoError(t, err)

	err = svc.SetRolePermissions(context.Background(), role.ID, []string{"user.delete"})
	require.NoError(t, err)

	var stored models.Role
	require.NoError(t, db.Preload("Permissions").First(&stored, "id = ?", role.ID).Error)

	var ids []string
	for _, perm := range stored.Permissions {
		ids = append(ids, perm.ID)
	}
	require.ElementsMatch(t, []string{"user.delete", "user.edit", "user.view"}, ids)
}

func TestPermissionService_SetRolePermissionsRejectsUnknown(t *testing.T) {
	_, svc := setupPermissionServiceTest(t)

	role, err := svc.CreateRole(context.Background(), CreateRoleInput{
		Name: "Unknown Permission Role",
	})
	require.NoError(t, err)

	err = svc.SetRolePermissions(context.Background(), role.ID, []string{"missing.permission"})
	require.Error(t, err)
	require.ErrorContains(t, err, permissions.ErrUnknownPermission.Error())
}

func TestPermissionService_UpdateRole(t *testing.T) {
	_, svc := setupPermissionServiceTest(t)

	role, err := svc.CreateRole(context.Background(), CreateRoleInput{
		Name:        "Initial",
		Description: "Initial description",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateRole(context.Background(), role.ID, UpdateRoleInput{
		Name:        "Updated",
		Description: "Updated description",
	})
	require.NoError(t, err)
	require.Equal(t, "Updated", updated.Name)
	require.Equal(t, "Updated description", updated.Description)
}

func setupPermissionServiceTest(t *testing.T) (*gorm.DB, *PermissionService) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.Role{},
		&models.Permission{},
	))
	require.NoError(t, permissions.Sync(context.Background(), db))

	svc, err := NewPermissionService(db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db, svc
}
