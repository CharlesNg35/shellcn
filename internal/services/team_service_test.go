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

func TestTeamServiceSetRoles(t *testing.T) {
	db := openTeamServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	teamSvc, err := NewTeamService(db, auditSvc)
	require.NoError(t, err)

	ctx := context.Background()

	team, err := teamSvc.Create(ctx, CreateTeamInput{
		Name: "Engineering",
	})
	require.NoError(t, err)

	roleA := models.Role{
		BaseModel: models.BaseModel{ID: "role.engineer"},
		Name:      "Engineer",
	}
	roleB := models.Role{
		BaseModel: models.BaseModel{ID: "role.devops"},
		Name:      "DevOps",
	}
	require.NoError(t, db.Create(&roleA).Error)
	require.NoError(t, db.Create(&roleB).Error)

	roles, err := teamSvc.SetRoles(ctx, team.ID, []string{roleA.ID, roleB.ID})
	require.NoError(t, err)
	require.Len(t, roles, 2)

	saved, err := teamSvc.ListRoles(ctx, team.ID)
	require.NoError(t, err)
	require.Len(t, saved, 2)

	roles, err = teamSvc.SetRoles(ctx, team.ID, []string{roleB.ID})
	require.NoError(t, err)
	require.Len(t, roles, 1)

	_, err = teamSvc.SetRoles(ctx, team.ID, []string{"does-not-exist"})
	require.Error(t, err)
}
