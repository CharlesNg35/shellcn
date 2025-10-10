package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestUserServiceCreateGetAndList(t *testing.T) {
	_, userSvc, auditSvc := setupUserServiceTest(t)
	ctx := context.Background()

	user, err := userSvc.Create(ctx, CreateUserInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "S3cret!!",
	})
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)
	require.NotEqual(t, "S3cret!!", user.Password)
	require.True(t, user.IsActive)

	fetched, err := userSvc.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "alice", fetched.Username)

	users, total, err := userSvc.List(ctx, ListUsersOptions{
		Page:     1,
		PageSize: 5,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	require.Equal(t, user.ID, users[0].ID)

	// Ensure audit log captured the create event.
	logs, _, err := auditSvc.List(ctx, AuditListOptions{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
}

func TestUserServiceUpdateAndActivation(t *testing.T) {
	_, userSvc, _ := setupUserServiceTest(t)
	ctx := context.Background()

	user, err := userSvc.Create(ctx, CreateUserInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "Password1!",
	})
	require.NoError(t, err)

	newEmail := "bob.new@example.com"
	firstName := "Bob"
	lastName := "Builder"
	updated, err := userSvc.Update(ctx, user.ID, UpdateUserInput{
		Email:     &newEmail,
		FirstName: &firstName,
		LastName:  &lastName,
	})
	require.NoError(t, err)
	require.Equal(t, newEmail, updated.Email)
	require.Equal(t, firstName, updated.FirstName)
	require.Equal(t, lastName, updated.LastName)

	require.NoError(t, userSvc.SetActive(ctx, user.ID, false))

	fetched, err := userSvc.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, fetched.IsActive)
}

func TestUserServiceRootProtectionAndPasswordChange(t *testing.T) {
	_, userSvc, _ := setupUserServiceTest(t)
	ctx := context.Background()

	rootUser, err := userSvc.Create(ctx, CreateUserInput{
		Username: "root",
		Email:    "root@example.com",
		Password: "StrongPass!",
		IsRoot:   true,
	})
	require.NoError(t, err)

	err = userSvc.SetActive(ctx, rootUser.ID, false)
	require.ErrorIs(t, err, ErrRootUserImmutable)

	err = userSvc.Delete(ctx, rootUser.ID)
	require.ErrorIs(t, err, ErrRootUserImmutable)

	// Non-root password change
	user, err := userSvc.Create(ctx, CreateUserInput{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "Initial1!",
	})
	require.NoError(t, err)

	require.NoError(t, userSvc.ChangePassword(ctx, user.ID, "NewPass2!"))

	updated, err := userSvc.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotEqual(t, user.Password, updated.Password)
}

func setupUserServiceTest(t *testing.T) (*gorm.DB, *UserService, *AuditService) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.Team{},
		&models.Role{},
		&models.Permission{},
		&models.User{},
		&models.AuditLog{},
	))

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := NewUserService(db, auditSvc)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db, userSvc, auditSvc
}
