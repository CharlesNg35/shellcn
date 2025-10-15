package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestSessionLifecycle_StartAndCloseSession(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)
	mockChat := &mockChatStore{}

	start := time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC)

	seedSessionFixtures(t, db, "conn-1", "user-1")

	svc, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
		WithSessionChatStore(mockChat),
		WithLifecycleClock(func() time.Time { return start }),
	)
	require.NoError(t, err)

	session, err := svc.StartSession(context.Background(), StartSessionParams{
		SessionID:      "sess-1",
		ConnectionID:   "conn-1",
		ConnectionName: "Primary SSH",
		ProtocolID:     "ssh",
		OwnerUserID:    "user-1",
		OwnerUserName:  "alice",
		Metadata:       map[string]any{"foo": "bar"},
		Host:           "10.0.0.1",
		Port:           22,
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, SessionStatusActive, session.Status)
	require.Equal(t, start, session.StartedAt)

	// Participant row created for owner
	var participant models.ConnectionSessionParticipant
	require.NoError(t, db.First(&participant, "session_id = ? AND user_id = ?", session.ID, "user-1").Error)
	require.Equal(t, "owner", participant.Role)
	require.Equal(t, "write", participant.AccessMode)
	require.NotZero(t, participant.JoinedAt)

	// Active session registered
	activeSession, ok := active.GetSession(session.ID)
	require.True(t, ok)
	require.Equal(t, "user-1", activeSession.OwnerUserID)
	require.Equal(t, "user-1", activeSession.WriteHolder)

	_, err = active.AppendChatMessage(session.ID, ActiveSessionChatMessage{
		AuthorID: "user-1",
		Author:   "alice",
		Content:  "hello world",
	})
	require.NoError(t, err)

	require.NoError(t, svc.CloseSession(context.Background(), CloseSessionParams{
		SessionID: session.ID,
		Reason:    "user_exit",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	}))

	var refreshed models.ConnectionSession
	require.NoError(t, db.First(&refreshed, "id = ?", session.ID).Error)
	require.Equal(t, SessionStatusClosed, refreshed.Status)
	require.NotNil(t, refreshed.ClosedAt)

	require.Equal(t, 1, len(mockChat.messages))
	require.Equal(t, "hello world", mockChat.messages[0].Content)

	_, stillActive := active.GetSession(session.ID)
	require.False(t, stillActive)
}

func TestSessionLifecycle_WriteDelegationAndRemoval(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)

	seedSessionFixtures(t, db, "conn-1", "user-1")

	svc, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
		WithLifecycleClock(func() time.Time { return time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC) }),
	)
	require.NoError(t, err)

	session, err := svc.StartSession(context.Background(), StartSessionParams{
		SessionID:     "sess-write",
		ConnectionID:  "conn-1",
		ProtocolID:    "ssh",
		OwnerUserID:   "user-1",
		OwnerUserName: "alice",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)

	createTestUser(t, db, "user-2", "bob")

	joined, err := svc.AddParticipant(context.Background(), AddParticipantParams{
		SessionID: session.ID,
		UserID:    "user-2",
		UserName:  "bob",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "read", joined.AccessMode)

	granted, err := svc.GrantWriteAccess(context.Background(), GrantWriteParams{
		SessionID:       session.ID,
		UserID:          "user-2",
		GrantedByUserID: ptrString("user-1"),
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "write", granted.AccessMode)

	dbParticipant := fetchParticipant(t, db, session.ID, "user-2")
	require.Equal(t, "write", dbParticipant.AccessMode)
	require.Equal(t, ptrString("user-1"), dbParticipant.GrantedByUserID)

	ownerParticipant := fetchParticipant(t, db, session.ID, "user-1")
	require.Equal(t, "read", ownerParticipant.AccessMode)

	sessionCopy, ok := active.GetSession(session.ID)
	require.True(t, ok)
	require.Equal(t, "user-2", sessionCopy.WriteHolder)

	removed, err := svc.RemoveParticipant(context.Background(), RemoveParticipantParams{
		SessionID: session.ID,
		UserID:    "user-2",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)
	require.True(t, removed)

	sessionCopy, ok = active.GetSession(session.ID)
	require.True(t, ok)
	require.Equal(t, "user-1", sessionCopy.WriteHolder)
	require.NotContains(t, sessionCopy.Participants, "user-2")

	dbParticipant = fetchParticipant(t, db, session.ID, "user-2")
	require.NotNil(t, dbParticipant.LeftAt)
}

func TestSessionLifecycle_RelinquishWriteAccess(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)
	seedSessionFixtures(t, db, "conn-1", "user-1")

	svc, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
		WithLifecycleClock(func() time.Time { return time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC) }),
	)
	require.NoError(t, err)

	session, err := svc.StartSession(context.Background(), StartSessionParams{
		SessionID:     "sess-release",
		ConnectionID:  "conn-1",
		ProtocolID:    "ssh",
		OwnerUserID:   "user-1",
		OwnerUserName: "alice",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)

	createTestUser(t, db, "user-2", "bob")

	_, err = svc.AddParticipant(context.Background(), AddParticipantParams{
		SessionID: session.ID,
		UserID:    "user-2",
		UserName:  "bob",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)

	_, err = svc.GrantWriteAccess(context.Background(), GrantWriteParams{
		SessionID:       session.ID,
		UserID:          "user-2",
		GrantedByUserID: ptrString("user-1"),
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)

	released, newWriter, err := svc.RelinquishWriteAccess(context.Background(), RelinquishWriteParams{
		SessionID: session.ID,
		UserID:    "user-2",
		Actor: SessionActor{
			UserID:   "user-2",
			Username: "bob",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "user-2", released.UserID)
	require.Equal(t, "read", released.AccessMode)
	require.NotNil(t, newWriter)
	require.Equal(t, "user-1", newWriter.UserID)
	require.Equal(t, "write", newWriter.AccessMode)

	participant := fetchParticipant(t, db, session.ID, "user-2")
	require.Equal(t, "read", participant.AccessMode)
	require.Nil(t, participant.GrantedByUserID)

	ownerParticipant := fetchParticipant(t, db, session.ID, "user-1")
	require.Equal(t, "write", ownerParticipant.AccessMode)
	require.NotNil(t, ownerParticipant.GrantedByUserID)
	require.Equal(t, ptrString("user-2"), ownerParticipant.GrantedByUserID)

	releasedOwner, newWriter, err := svc.RelinquishWriteAccess(context.Background(), RelinquishWriteParams{
		SessionID: session.ID,
		UserID:    "user-1",
		Actor: SessionActor{
			UserID:   "user-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "user-1", releasedOwner.UserID)
	require.Equal(t, "read", releasedOwner.AccessMode)
	require.Nil(t, newWriter)

	ownerParticipant = fetchParticipant(t, db, session.ID, "user-1")
	require.Equal(t, "read", ownerParticipant.AccessMode)
	require.Nil(t, ownerParticipant.GrantedByUserID)
}

func TestSessionLifecycle_AuthorizeSessionAccess(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)
	seedSessionFixtures(t, db, "conn-1", "owner-1")
	createTestUser(t, db, "participant-1", "bob")

	svc, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
	)
	require.NoError(t, err)

	session, err := svc.StartSession(context.Background(), StartSessionParams{
		SessionID:     "sess-access",
		ConnectionID:  "conn-1",
		ProtocolID:    "ssh",
		OwnerUserID:   "owner-1",
		OwnerUserName: "alice",
		Actor: SessionActor{
			UserID:   "owner-1",
			Username: "alice",
		},
	})
	require.NoError(t, err)

	_, err = svc.AuthorizeSessionAccess(context.Background(), session.ID, "owner-1")
	require.NoError(t, err)

	_, err = svc.AddParticipant(context.Background(), AddParticipantParams{
		SessionID: session.ID,
		UserID:    "participant-1",
		UserName:  "bob",
		Actor:     SessionActor{UserID: "owner-1", Username: "alice"},
	})
	require.NoError(t, err)

	_, err = svc.AuthorizeSessionAccess(context.Background(), session.ID, "participant-1")
	require.NoError(t, err)

	_, err = svc.AuthorizeSessionAccess(context.Background(), session.ID, "someone-else")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSessionAccessDenied)

	_, err = svc.AuthorizeSessionAccess(context.Background(), "missing", "owner-1")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func fetchParticipant(t *testing.T, db *gorm.DB, sessionID, userID string) models.ConnectionSessionParticipant {
	t.Helper()
	var model models.ConnectionSessionParticipant
	require.NoError(t, db.First(&model, "session_id = ? AND user_id = ?", sessionID, userID).Error)
	return model
}

type mockChatStore struct {
	messages []ActiveSessionChatMessage
}

func (m *mockChatStore) PersistMessages(_ context.Context, _ string, messages []ActiveSessionChatMessage) error {
	m.messages = append(m.messages, messages...)
	return nil
}

func seedSessionFixtures(t *testing.T, db *gorm.DB, connectionID, ownerID string) {
	t.Helper()

	user := models.User{
		BaseModel: models.BaseModel{ID: ownerID},
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	conn := models.Connection{
		BaseModel:   models.BaseModel{ID: connectionID},
		Name:        "Primary SSH",
		ProtocolID:  "ssh",
		OwnerUserID: ownerID,
	}
	require.NoError(t, db.Create(&conn).Error)
}

func createTestUser(t *testing.T, db *gorm.DB, id, username string) {
	t.Helper()
	user := models.User{
		BaseModel: models.BaseModel{ID: id},
		Username:  username,
		Email:     username + "@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)
}
