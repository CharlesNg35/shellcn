package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
)

func TestUserPreferencesService_Get_Defaults(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := NewUserService(db, auditSvc)
	require.NoError(t, err)

	user, err := userSvc.Create(context.Background(), CreateUserInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	prefSvc, err := NewUserPreferencesService(db, auditSvc)
	require.NoError(t, err)

	prefs, err := prefSvc.Get(context.Background(), user.ID)
	require.NoError(t, err)

	require.Equal(t, DefaultUserPreferences(), prefs)
}

func TestUserPreferencesService_Update(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := NewUserService(db, auditSvc)
	require.NoError(t, err)

	user, err := userSvc.Create(context.Background(), CreateUserInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	prefSvc, err := NewUserPreferencesService(db, auditSvc)
	require.NoError(t, err)

	update := UserPreferences{
		SSH: SSHPreferences{
			Terminal: SSHTerminalPreferences{
				FontFamily:   "JetBrains Mono",
				CursorStyle:  "beam",
				CopyOnSelect: false,
			},
			SFTP: SSHSFTPPreferences{
				ShowHiddenFiles: true,
				AutoOpenQueue:   false,
			},
		},
	}

	updated, err := prefSvc.Update(context.Background(), user.ID, update)
	require.NoError(t, err)
	require.Equal(t, false, updated.SSH.Terminal.CopyOnSelect)
	require.Equal(t, "beam", updated.SSH.Terminal.CursorStyle)
	require.True(t, updated.SSH.SFTP.ShowHiddenFiles)

	stored, err := prefSvc.Get(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, updated, stored)
}
