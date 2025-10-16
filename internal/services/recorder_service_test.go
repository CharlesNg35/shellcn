package services

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestRecorderService_RecordLifecycle(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	policy := RecorderPolicy{
		Mode:           RecordingModeForced,
		Storage:        "filesystem",
		RetentionDays:  30,
		RequireConsent: false,
	}

	service, err := NewRecorderService(db, store, WithRecorderPolicy(policy))
	require.NoError(t, err)

	owner := createRecorderTestUser(t, db, "owner")
	connection := createRecorderTestConnection(t, db, "conn-1", owner.ID)

	startedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	session := models.ConnectionSession{
		BaseModel:    models.BaseModel{ID: "sess-record"},
		ConnectionID: connection.ID,
		ProtocolID:   "ssh",
		OwnerUserID:  owner.ID,
		Status:       SessionStatusActive,
		StartedAt:    startedAt,
		Metadata: datatypes.JSON([]byte(`{
			"recording_enabled": true,
			"terminal_width": 120,
			"terminal_height": 40,
			"terminal_type": "xterm-256color"
		}`)),
	}
	require.NoError(t, db.Create(&session).Error)

	ctx := context.Background()
	require.NoError(t, service.OnSessionStarted(ctx, &session))

	status, err := service.Status(ctx, session.ID)
	require.NoError(t, err)
	require.True(t, status.Active)
	require.Equal(t, session.ID, status.SessionID)
	require.Greater(t, status.BytesRecorded, int64(0))

	service.RecordStream(session.ID, "stdout", []byte("hello world\n"))
	service.RecordStream(session.ID, "stderr", []byte("oops\n"))

	// Finalise via session close.
	session.ClosedAt = recordingTimePtr(startedAt.Add(5 * time.Second))
	session.Status = SessionStatusClosed
	require.NoError(t, service.OnSessionClosed(ctx, &session, "completed"))

	status, err = service.Status(ctx, session.ID)
	require.NoError(t, err)
	require.False(t, status.Active)
	require.NotEmpty(t, status.RecordID)
	require.NotEmpty(t, status.StoragePath)

	var records []models.ConnectionSessionRecord
	require.NoError(t, db.Find(&records).Error)
	require.Len(t, records, 1)
	record := records[0]
	require.Equal(t, session.ID, record.SessionID)
	require.Equal(t, policy.Storage, record.StorageKind)
	require.NotZero(t, record.SizeBytes)
	require.NotEmpty(t, record.Checksum)
	require.NotNil(t, record.RetentionUntil)

	reader, fetchedRecord, err := service.OpenRecording(ctx, record.ID)
	require.NoError(t, err)
	defer reader.Close()
	require.Equal(t, record.ID, fetchedRecord.ID)

	gzr, err := gzip.NewReader(reader)
	require.NoError(t, err)
	defer gzr.Close()

	scanner := bufio.NewScanner(gzr)
	require.True(t, scanner.Scan())
	var header map[string]any
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &header))
	require.EqualValues(t, 2, header["version"])
	require.EqualValues(t, 120, header["width"])
	require.EqualValues(t, 40, header["height"])

	lines := 0
	for scanner.Scan() {
		lines++
	}
	require.GreaterOrEqual(t, lines, 2, "expected at least stdout/stderr events")
	require.NoError(t, scanner.Err())
}

func TestRecorderService_StopRecording(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	service, err := NewRecorderService(db, store, WithRecorderPolicy(RecorderPolicy{
		Mode:           RecordingModeForced,
		Storage:        "filesystem",
		RetentionDays:  0,
		RequireConsent: false,
	}))
	require.NoError(t, err)

	owner := createRecorderTestUser(t, db, "owner-stop")
	connection := createRecorderTestConnection(t, db, "conn-stop", owner.ID)

	session := models.ConnectionSession{
		BaseModel:    models.BaseModel{ID: "sess-stop"},
		ConnectionID: connection.ID,
		ProtocolID:   "ssh",
		OwnerUserID:  owner.ID,
		Status:       SessionStatusActive,
		StartedAt:    time.Now().UTC(),
	}
	require.NoError(t, db.Create(&session).Error)

	require.NoError(t, service.OnSessionStarted(context.Background(), &session))
	service.RecordStream(session.ID, "stdout", []byte("data\n"))

	record, err := service.StopRecording(context.Background(), session.ID, owner.ID, "manual")
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, owner.ID, record.CreatedByUserID)

	require.NoError(t, service.OnSessionClosed(context.Background(), &session, "completed"))

	status, err := service.Status(context.Background(), session.ID)
	require.NoError(t, err)
	require.False(t, status.Active)
	require.Equal(t, record.ID, status.RecordID)
}

func TestRecorderService_OptionalModeRequiresOptIn(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	service, err := NewRecorderService(db, store, WithRecorderPolicy(RecorderPolicy{
		Mode:           RecordingModeOptional,
		Storage:        "filesystem",
		RetentionDays:  0,
		RequireConsent: true,
	}))
	require.NoError(t, err)

	owner := createRecorderTestUser(t, db, "owner-opt")
	connection := createRecorderTestConnection(t, db, "conn-opt", owner.ID)

	session := models.ConnectionSession{
		BaseModel:    models.BaseModel{ID: "sess-optional"},
		ConnectionID: connection.ID,
		ProtocolID:   "ssh",
		OwnerUserID:  owner.ID,
		Status:       SessionStatusActive,
		StartedAt:    time.Now().UTC(),
		Metadata:     datatypes.JSON([]byte(`{"recording_enabled": false}`)),
	}

	require.NoError(t, db.Create(&session).Error)
	require.NoError(t, service.OnSessionStarted(context.Background(), &session))

	status, err := service.Status(context.Background(), session.ID)
	require.NoError(t, err)
	require.False(t, status.Active)

	service.RecordStream(session.ID, "stdout", []byte("ignored\n"))
	require.NoError(t, service.OnSessionClosed(context.Background(), &session, "done"))

	var count int64
	require.NoError(t, db.Model(&models.ConnectionSessionRecord{}).
		Where("session_id = ?", session.ID).
		Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func recordingTimePtr(t time.Time) *time.Time {
	return &t
}

func createRecorderTestUser(t *testing.T, db *gorm.DB, username string) *models.User {
	userSvc, err := NewUserService(db, nil)
	require.NoError(t, err)

	user, err := userSvc.Create(context.Background(), CreateUserInput{
		Username: username,
		Email:    username + "@example.com",
		Password: "password",
	})
	require.NoError(t, err)
	return user
}

func createRecorderTestConnection(t *testing.T, db *gorm.DB, id, ownerID string) *models.Connection {
	conn := &models.Connection{
		BaseModel:   models.BaseModel{ID: id},
		Name:        "Recorder Connection",
		ProtocolID:  "ssh",
		OwnerUserID: ownerID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(conn).Error)
	return conn
}

func TestRecorderService_ListRecordings(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	service, err := NewRecorderService(db, store)
	require.NoError(t, err)

	owner := createRecorderTestUser(t, db, "owner-list")
	otherUser := createRecorderTestUser(t, db, "other-list")

	team := &models.Team{Name: "DevOps"}
	require.NoError(t, db.Create(team).Error)
	require.NoError(t, db.Model(team).Association("Users").Append(owner))

	teamConnID := "conn-team"
	connTeam := &models.Connection{
		BaseModel:   models.BaseModel{ID: teamConnID},
		Name:        "Team Connection",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
		TeamID:      &team.ID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(connTeam).Error)

	personalConn := createRecorderTestConnection(t, db, "conn-personal", owner.ID)

	otherTeam := &models.Team{Name: "Security"}
	require.NoError(t, db.Create(otherTeam).Error)
	require.NoError(t, db.Model(otherTeam).Association("Users").Append(otherUser))

	connOtherTeam := &models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-other-team"},
		Name:        "Other Team",
		ProtocolID:  "ssh",
		OwnerUserID: otherUser.ID,
		TeamID:      &otherTeam.ID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(connOtherTeam).Error)

	now := time.Now().UTC()
	sessionTeam := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-team"},
		ConnectionID:    connTeam.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     owner.ID,
		TeamID:          &team.ID,
		Status:          SessionStatusClosed,
		StartedAt:       now.Add(-2 * time.Hour),
		LastHeartbeatAt: now.Add(-2 * time.Hour),
	}
	sessionPersonal := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-personal"},
		ConnectionID:    personalConn.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     owner.ID,
		Status:          SessionStatusClosed,
		StartedAt:       now.Add(-time.Hour),
		LastHeartbeatAt: now.Add(-time.Hour),
	}
	sessionOther := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-other"},
		ConnectionID:    connOtherTeam.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     otherUser.ID,
		TeamID:          &otherTeam.ID,
		Status:          SessionStatusClosed,
		StartedAt:       now.Add(-3 * time.Hour),
		LastHeartbeatAt: now.Add(-3 * time.Hour),
	}

	require.NoError(t, db.Create(&sessionTeam).Error)
	require.NoError(t, db.Create(&sessionPersonal).Error)
	require.NoError(t, db.Create(&sessionOther).Error)

	records := []models.ConnectionSessionRecord{
		{
			BaseModel:       models.BaseModel{ID: "rec-team"},
			SessionID:       sessionTeam.ID,
			StorageKind:     "filesystem",
			StoragePath:     "team.cast.gz",
			SizeBytes:       150,
			DurationSeconds: 90,
			CreatedByUserID: owner.ID,
		},
		{
			BaseModel:       models.BaseModel{ID: "rec-personal"},
			SessionID:       sessionPersonal.ID,
			StorageKind:     "filesystem",
			StoragePath:     "personal.cast.gz",
			SizeBytes:       200,
			DurationSeconds: 120,
			CreatedByUserID: owner.ID,
		},
		{
			BaseModel:       models.BaseModel{ID: "rec-other"},
			SessionID:       sessionOther.ID,
			StorageKind:     "filesystem",
			StoragePath:     "other.cast.gz",
			SizeBytes:       90,
			DurationSeconds: 60,
			CreatedByUserID: otherUser.ID,
		},
	}
	for _, record := range records {
		require.NoError(t, db.Create(&record).Error)
	}

	t.Run("personal scope includes owned sessions", func(t *testing.T) {
		summaries, total, err := service.ListRecordings(context.Background(), ListRecordingsOptions{
			Scope:  RecordingScopePersonal,
			UserID: owner.ID,
			Limit:  10,
		})
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, summaries, 2)
		gotIDs := []string{summaries[0].RecordID, summaries[1].RecordID}
		require.ElementsMatch(t, []string{"rec-personal", "rec-team"}, gotIDs)
	})

	t.Run("team scope filters by membership", func(t *testing.T) {
		summaries, total, err := service.ListRecordings(context.Background(), ListRecordingsOptions{
			Scope:   RecordingScopeTeam,
			UserID:  owner.ID,
			TeamIDs: []string{team.ID},
			Limit:   10,
		})
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, summaries, 2)
	})

	t.Run("team scope with explicit team filter", func(t *testing.T) {
		summaries, total, err := service.ListRecordings(context.Background(), ListRecordingsOptions{
			Scope:   RecordingScopeTeam,
			UserID:  owner.ID,
			TeamIDs: []string{team.ID},
			TeamID:  team.ID,
			Limit:   10,
		})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, summaries, 1)
		require.Equal(t, "rec-team", summaries[0].RecordID)
	})

	t.Run("all scope returns every record", func(t *testing.T) {
		summaries, total, err := service.ListRecordings(context.Background(), ListRecordingsOptions{
			Scope:  RecordingScopeAll,
			UserID: owner.ID,
			Limit:  10,
		})
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		require.Len(t, summaries, 3)
	})

	t.Run("all scope filters by team", func(t *testing.T) {
		summaries, total, err := service.ListRecordings(context.Background(), ListRecordingsOptions{
			Scope:  RecordingScopeAll,
			UserID: owner.ID,
			TeamID: otherTeam.ID,
			Limit:  10,
		})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, summaries, 1)
		require.Equal(t, "rec-other", summaries[0].RecordID)
	})
}

func TestRecorderService_CleanupExpired(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	root := filepath.Join(t.TempDir(), "records")
	store, err := NewFilesystemRecorderStore(root)
	require.NoError(t, err)

	recorder, err := NewRecorderService(db, store)
	require.NoError(t, err)

	owner := createRecorderTestUser(t, db, "owner")

	retention := time.Now().Add(-24 * time.Hour)
	path := "expired.cast.gz"
	filePath := filepath.Join(root, path)
	require.NoError(t, os.WriteFile(filePath, []byte("recording"), 0o600))
	require.NoError(t, db.Create(&models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-cleanup"},
		Name:        "Cleanup",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}).Error)
	require.NoError(t, db.Create(&models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-cleanup"},
		ConnectionID:    "conn-cleanup",
		ProtocolID:      "ssh",
		OwnerUserID:     owner.ID,
		Status:          SessionStatusClosed,
		StartedAt:       time.Now(),
		LastHeartbeatAt: time.Now(),
	}).Error)

	record := models.ConnectionSessionRecord{
		BaseModel:       models.BaseModel{ID: "rec-expired"},
		SessionID:       "sess-cleanup",
		StorageKind:     "filesystem",
		StoragePath:     path,
		SizeBytes:       9,
		RetentionUntil:  &retention,
		CreatedByUserID: owner.ID,
	}
	require.NoError(t, db.Create(&record).Error)

	deleted, err := recorder.CleanupExpired(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	var count int64
	require.NoError(t, db.Model(&models.ConnectionSessionRecord{}).Count(&count).Error)
	require.Equal(t, int64(0), count)

	_, statErr := os.Stat(filePath)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}
