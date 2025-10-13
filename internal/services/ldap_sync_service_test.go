package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestLDAPSyncServiceSyncGroups(t *testing.T) {
	db := openLDAPSyncTestDB(t)

	jwtService, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "sync-secret", AccessTokenTTL: time.Hour})
	require.NoError(t, err)

	sessionService, err := iauth.NewSessionService(db, jwtService, iauth.SessionConfig{})
	require.NoError(t, err)

	ssoManager, err := iauth.NewSSOManager(db, sessionService, iauth.SSOConfig{})
	require.NoError(t, err)

	syncSvc, err := NewLDAPSyncService(db, ssoManager)
	require.NoError(t, err)

	hashed, err := crypto.HashPassword("password")
	require.NoError(t, err)

	user := &models.User{
		Username:     "alice",
		Email:        "alice@example.com",
		Password:     hashed,
		AuthProvider: "ldap",
		IsActive:     true,
	}
	require.NoError(t, db.Create(user).Error)

	cfg := models.LDAPConfig{
		SyncGroups:           true,
		GroupBaseDN:          "ou=Groups,dc=example,dc=com",
		GroupFilter:          "(objectClass=nestedGroup)",
		GroupMemberAttribute: "member",
		GroupNameAttribute:   "cn",
		AttributeMapping: map[string]string{
			"groups": "memberOf",
		},
	}

	result, err := syncSvc.SyncGroups(context.Background(), cfg, user, []string{
		"cn=Engineering,ou=Groups,dc=example,dc=com",
		"QA",
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.TeamsCreated)
	require.Equal(t, 2, result.MembershipsAdded)
	require.Equal(t, 0, result.MembershipsRemoved)

	var teamCount int64
	require.NoError(t, db.Model(&models.Team{}).Where("source = ?", "ldap").Count(&teamCount).Error)
	require.Equal(t, int64(2), teamCount)

	var membershipCount int64
	require.NoError(t, db.Table("user_teams").Where("user_id = ?", user.ID).Count(&membershipCount).Error)
	require.Equal(t, int64(2), membershipCount)

	result, err = syncSvc.SyncGroups(context.Background(), cfg, user, []string{"QA"})
	require.NoError(t, err)
	require.Equal(t, 0, result.TeamsCreated)
	require.Equal(t, 0, result.MembershipsAdded)
	require.Equal(t, 1, result.MembershipsRemoved)

	require.NoError(t, db.Table("user_teams").Where("user_id = ?", user.ID).Count(&membershipCount).Error)
	require.Equal(t, int64(1), membershipCount)
}

func TestLDAPSyncServiceSyncFromIdentities(t *testing.T) {
	db := openLDAPSyncTestDB(t)

	jwtService, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "sync-secret", AccessTokenTTL: time.Hour})
	require.NoError(t, err)

	sessionService, err := iauth.NewSessionService(db, jwtService, iauth.SessionConfig{})
	require.NoError(t, err)

	ssoManager, err := iauth.NewSSOManager(db, sessionService, iauth.SSOConfig{})
	require.NoError(t, err)

	syncSvc, err := NewLDAPSyncService(db, ssoManager)
	require.NoError(t, err)

	cfg := models.LDAPConfig{
		SyncGroups:           true,
		GroupBaseDN:          "ou=Groups,dc=example,dc=com",
		GroupFilter:          "(objectClass=nestedGroup)",
		GroupMemberAttribute: "member",
		GroupNameAttribute:   "cn",
		AttributeMapping: map[string]string{
			"groups": "memberOf",
		},
	}

	identity := providers.Identity{
		Provider:  "ldap",
		Subject:   "cn=Alice,ou=People,dc=example,dc=com",
		Email:     "alice@example.com",
		FirstName: "Alice",
		LastName:  "Smith",
		Groups: []string{
			"cn=Engineering,ou=Groups,dc=example,dc=com",
		},
	}

	summary, err := syncSvc.SyncFromIdentities(context.Background(), cfg, []providers.Identity{identity}, true)
	require.NoError(t, err)
	require.Equal(t, 1, summary.UsersCreated)
	require.Equal(t, 1, summary.TeamsCreated)
	require.Equal(t, 1, summary.MembershipsAdded)

	var user models.User
	require.NoError(t, db.Take(&user, "LOWER(email) = ?", "alice@example.com").Error)
	require.Equal(t, "ldap", user.AuthProvider)

	var membershipCount int64
	require.NoError(t, db.Table("user_teams").Where("user_id = ?", user.ID).Count(&membershipCount).Error)
	require.Equal(t, int64(1), membershipCount)

	summary, err = syncSvc.SyncFromIdentities(context.Background(), cfg, []providers.Identity{identity}, false)
	require.NoError(t, err)
	require.Equal(t, 1, summary.UsersUpdated)
}

func openLDAPSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Team{},
	))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
