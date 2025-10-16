package services

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/database/testutil"
)

func TestProtocolSettingsService_UpdateSSHSettings(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	recorder, err := NewRecorderService(db, store)
	require.NoError(t, err)

	svc, err := NewProtocolSettingsService(db, auditSvc, WithProtocolRecorder(recorder))
	require.NoError(t, err)

	actor := SessionActor{UserID: "admin", Username: "Admin"}

	settings, err := svc.UpdateSSHSettings(context.Background(), actor, UpdateSSHSettingsInput{
		Session: SessionSettingsInput{
			ConcurrentLimit:    5,
			IdleTimeoutMinutes: 60,
			EnableSFTP:         true,
		},
		Terminal: TerminalSettingsInput{
			ThemeMode:   "force_dark",
			FontFamily:  "JetBrains Mono",
			FontSize:    16,
			Scrollback:  1500,
			EnableWebGL: false,
		},
		Recording: RecordingSettingsInput{
			Mode:           RecordingModeForced,
			Storage:        "filesystem",
			RetentionDays:  45,
			RequireConsent: false,
		},
		Collaboration: CollaborationSettingsInput{
			AllowSharing:          true,
			RestrictWriteToAdmins: true,
		},
	})
	require.NoError(t, err)

	require.Equal(t, 5, settings.Session.ConcurrentLimit)
	require.Equal(t, 60, settings.Session.IdleTimeoutMinutes)
	require.True(t, settings.Session.EnableSFTP)
	require.Equal(t, "force_dark", settings.Terminal.ThemeMode)
	require.Equal(t, "JetBrains Mono", settings.Terminal.FontFamily)
	require.Equal(t, 16, settings.Terminal.FontSize)
	require.Equal(t, 1500, settings.Terminal.Scrollback)
	require.False(t, settings.Terminal.EnableWebGL)
	require.True(t, settings.Collaboration.RestrictWriteToAdmins)

	require.Equal(t, RecordingModeForced, settings.Recording.Mode)
	require.Equal(t, 45, settings.Recording.RetentionDays)
	require.False(t, settings.Recording.RequireConsent)

	policy := recorder.Policy()
	require.Equal(t, RecordingModeForced, policy.Mode)
	require.Equal(t, 45, policy.RetentionDays)

	modeValue, err := database.GetSystemSetting(context.Background(), db, "recording.mode")
	require.NoError(t, err)
	require.Equal(t, RecordingModeForced, modeValue)

	concurrentLimit, err := database.GetSystemSetting(context.Background(), db, "sessions.concurrent_limit_default")
	require.NoError(t, err)
	require.Equal(t, "5", concurrentLimit)

	themeMode, err := database.GetSystemSetting(context.Background(), db, "protocol.ssh.terminal.theme_mode")
	require.NoError(t, err)
	require.Equal(t, "force_dark", themeMode)

	restrictWrite, err := database.GetSystemSetting(context.Background(), db, "session_sharing.restrict_write_to_admins")
	require.NoError(t, err)
	require.Equal(t, "true", restrictWrite)
}

func TestProtocolSettingsService_GetSSHSettings_Default(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	svc, err := NewProtocolSettingsService(db, nil)
	require.NoError(t, err)

	settings, err := svc.GetSSHSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, RecordingModeOptional, settings.Recording.Mode)
	require.Equal(t, 0, settings.Session.ConcurrentLimit)
	require.True(t, settings.Session.EnableSFTP)
	require.Equal(t, "auto", settings.Terminal.ThemeMode)
	require.True(t, settings.Collaboration.AllowSharing)
}
