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
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
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
		Recording: RecordingSettingsInput{
			Mode:           RecordingModeForced,
			Storage:        "filesystem",
			RetentionDays:  45,
			RequireConsent: false,
		},
	})
	require.NoError(t, err)

	require.Equal(t, RecordingModeForced, settings.Recording.Mode)
	require.Equal(t, 45, settings.Recording.RetentionDays)
	require.False(t, settings.Recording.RequireConsent)

	policy := recorder.Policy()
	require.Equal(t, RecordingModeForced, policy.Mode)
	require.Equal(t, 45, policy.RetentionDays)

	modeValue, err := database.GetSystemSetting(context.Background(), db, "recording.mode")
	require.NoError(t, err)
	require.Equal(t, RecordingModeForced, modeValue)
}

func TestProtocolSettingsService_GetSSHSettings_Default(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	svc, err := NewProtocolSettingsService(db, nil)
	require.NoError(t, err)

	settings, err := svc.GetSSHSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, RecordingModeOptional, settings.Recording.Mode)
}
