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

	org := models.Organization{Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	hashed, err := crypto.HashPassword("p@ssW0rd!")
	require.NoError(t, err)

	user := models.User{
		Username:       "member",
		Email:          "member@example.com",
		Password:       hashed,
		OrganizationID: &org.ID,
	}
	require.NoError(t, db.Create(&user).Error)

	team, err := teamSvc.Create(ctx, CreateTeamInput{
		OrganizationID: org.ID,
		Name:           "Operations",
		Description:    "Ops team",
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

	org := models.Organization{Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	team, err := teamSvc.Create(ctx, CreateTeamInput{
		OrganizationID: org.ID,
		Name:           "Support",
	})
	require.NoError(t, err)

	name := "Customer Support"
	updated, err := teamSvc.Update(ctx, team.ID, UpdateTeamInput{Name: &name})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)

	teams, err := teamSvc.ListByOrganization(ctx, org.ID)
	require.NoError(t, err)
	require.Len(t, teams, 1)
}

func openTeamServiceTestDB(t *testing.T) *gorm.DB {
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
