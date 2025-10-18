package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestSeedSSHProtocolDefaults(t *testing.T) {
	db, err := database.Open(database.Config{Driver: "sqlite"})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))

	cfg := &app.Config{
		Features: app.FeatureConfig{
			Recording: app.RecordingConfig{
				Mode:           "optional",
				Storage:        "filesystem",
				RetentionDays:  60,
				RequireConsent: true,
			},
			SessionSharing: app.SessionSharingConfig{
				Enabled:               true,
				MaxSharedUsers:        6,
				AllowDefault:          true,
				RestrictWriteToAdmins: false,
			},
			Sessions: app.SessionLifecycleConfig{
				ConcurrentLimitDefault: 4,
				IdleTimeout:            45 * time.Minute,
			},
		},
		Protocols: app.ProtocolConfig{
			SSH: app.SSHProtocolConfig{
				Enabled:           true,
				EnableSFTPDefault: true,
				Terminal: app.SSHTerminalConfig{
					ThemeMode:  "auto",
					FontFamily: "monospace",
					FontSize:   14,
					Scrollback: 1500,
				},
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, seedSSHProtocolDefaults(ctx, db, cfg))
	require.NoError(t, seedRecordingDefaults(ctx, db, cfg))
	require.NoError(t, seedSessionSharingDefaults(ctx, db, cfg))

	assertSystemSetting(t, ctx, db, "protocol.ssh.enable_sftp_default", "true")
	assertSystemSetting(t, ctx, db, "recording.retention_days", "60")
	assertSystemSetting(t, ctx, db, "session_sharing.enabled", "true")
	assertSystemSetting(t, ctx, db, "session_sharing.max_shared_users", "6")
	assertSystemSetting(t, ctx, db, "session_sharing.allow_default", "true")
	assertSystemSetting(t, ctx, db, "session_sharing.restrict_write_to_admins", "false")
	assertSystemSetting(t, ctx, db, "sessions.concurrent_limit_default", "4")
	assertSystemSetting(t, ctx, db, "sessions.idle_timeout_minutes", "45")
	assertSystemSetting(t, ctx, db, "protocol.ssh.terminal.scrollback_limit", "1500")

	// Ensure existing values are preserved.
	require.NoError(t, database.UpsertSystemSetting(ctx, db, "protocol.ssh.enable_sftp_default", "false"))
	cfg.Protocols.SSH.EnableSFTPDefault = true
	require.NoError(t, seedSSHProtocolDefaults(ctx, db, cfg))
	value, err := database.GetSystemSetting(ctx, db, "protocol.ssh.enable_sftp_default")
	require.NoError(t, err)
	require.Equal(t, "false", value)

	// Ensure global recording values are preserved.
	require.NoError(t, database.UpsertSystemSetting(ctx, db, "recording.mode", "disabled"))
	cfg.Features.Recording.Mode = "forced"
	require.NoError(t, seedRecordingDefaults(ctx, db, cfg))
	recordingMode, err := database.GetSystemSetting(ctx, db, "recording.mode")
	require.NoError(t, err)
	require.Equal(t, "disabled", recordingMode)

	// Ensure session sharing defaults respect pre-existing values.
	require.NoError(t, database.UpsertSystemSetting(ctx, db, "session_sharing.allow_default", "false"))
	cfg.Features.SessionSharing.AllowDefault = true
	require.NoError(t, seedSessionSharingDefaults(ctx, db, cfg))
	allowDefault, err := database.GetSystemSetting(ctx, db, "session_sharing.allow_default")
	require.NoError(t, err)
	require.Equal(t, "false", allowDefault)

	// Ensure session lifecycle defaults respect pre-existing values.
	require.NoError(t, database.UpsertSystemSetting(ctx, db, "sessions.idle_timeout_minutes", "60"))
	cfg.Features.Sessions.IdleTimeout = 30 * time.Minute
	require.NoError(t, seedSessionSharingDefaults(ctx, db, cfg))
	idleTimeout, err := database.GetSystemSetting(ctx, db, "sessions.idle_timeout_minutes")
	require.NoError(t, err)
	require.Equal(t, "60", idleTimeout)
}

func assertSystemSetting(t *testing.T, ctx context.Context, db *gorm.DB, key, expected string) {
	t.Helper()
	value, err := database.GetSystemSetting(ctx, db, key)
	require.NoError(t, err)
	require.Equal(t, expected, value)
}
