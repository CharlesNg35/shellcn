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

func TestTeamServiceMembershipLifecycle(t *testing.T) {
	db := openTeamServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	teamSvc, err := NewTeamService(db, auditSvc)
	require.NoError(t, err)

	ctx := context.Background()

	hashed, err := crypto.HashPassword("p@ssW0rd!")
	require.NoError(t, err)

	user := models.User{
		Username: "member",
		Email:    "member@example.com",
		Password: hashed,
	}
	require.NoError(t, db.Create(&user).Error)

	team, err := teamSvc.Create(ctx, CreateTeamInput{
		Name:        "Operations",
		Description: "Ops team",
	})
	require.NoError(t, err)

	err = teamSvc.AddMember(ctx, team.ID, user.ID)
	require.NoError(t, err)

	members, err := teamSvc.ListMembers(ctx, team.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, user.ID, members[0].ID)

	err = teamSvc.AddMember(ctx, team.ID, user.ID)
	require.ErrorIs(t, err, ErrTeamMemberAlreadyExists)

	err = teamSvc.RemoveMember(ctx, team.ID, user.ID)
	require.NoError(t, err)

	err = teamSvc.RemoveMember(ctx, team.ID, user.ID)
	require.ErrorIs(t, err, ErrTeamMemberNotFound)
}

func TestTeamServiceUpdateAndList(t *testing.T) {
	db := openTeamServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	teamSvc, err := NewTeamService(db, auditSvc)
	require.NoError(t, err)

	ctx := context.Background()

	team, err := teamSvc.Create(ctx, CreateTeamInput{
		Name: "Support",
	})
	require.NoError(t, err)

	name := "Customer Support"
	updated, err := teamSvc.Update(ctx, team.ID, UpdateTeamInput{Name: &name})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)

	// Verify team was updated
	found, err := teamSvc.GetByID(ctx, team.ID)
	require.NoError(t, err)
	require.Equal(t, name, found.Name)
}

func openTeamServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
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
