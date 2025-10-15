package services

import (
	"context"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"gorm.io/gorm"
)

func TestSessionChatService_PostMessage(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)
	clock := time.Date(2024, 10, 2, 10, 0, 0, 0, time.UTC)
	lifecycle, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
		WithLifecycleClock(func() time.Time { return clock }),
	)
	require.NoError(t, err)

	createChatFixtures(t, db, "conn-1", "user-1")

	session, err := lifecycle.StartSession(context.Background(), StartSessionParams{
		SessionID:     "sess-chat-1",
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

	chatSvc, err := NewSessionChatService(db, active)
	require.NoError(t, err)

	message, err := chatSvc.PostMessage(context.Background(), ChatMessageParams{
		SessionID: session.ID,
		AuthorID:  "user-1",
		Author:    "alice",
		Content:   "<script>alert('xss')</script> hi",
	})
	require.NoError(t, err)
	require.NotEmpty(t, message.MessageID)
	require.Equal(t, html.EscapeString("<script>alert('xss')</script> hi"), message.Content)

	var stored models.ConnectionSessionMessage
	require.NoError(t, db.First(&stored, "id = ?", message.MessageID).Error)
	require.Equal(t, message.Content, stored.Content)
	require.Equal(t, session.ID, stored.SessionID)

	require.Empty(t, active.ConsumeChatBuffer(session.ID))
}

func TestSessionChatService_PersistMessages(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	active := NewActiveSessionService(nil)
	chatSvc, err := NewSessionChatService(db, active)
	require.NoError(t, err)

	// Seed session + user to satisfy FK constraints
	createChatFixtures(t, db, "conn-1", "user-1")
	require.NoError(t, db.Create(&models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-buffer"},
		ConnectionID:    "conn-1",
		ProtocolID:      "ssh",
		OwnerUserID:     "user-1",
		Status:          SessionStatusActive,
		StartedAt:       time.Now(),
		LastHeartbeatAt: time.Now(),
	}).Error)

	messages := []ActiveSessionChatMessage{
		{
			MessageID: "msg-1",
			SessionID: "sess-buffer",
			AuthorID:  "user-1",
			Content:   "first",
			CreatedAt: time.Now(),
		},
		{
			MessageID: "msg-2",
			SessionID: "sess-buffer",
			AuthorID:  "user-1",
			Content:   "second",
			CreatedAt: time.Now(),
		},
	}

	require.NoError(t, chatSvc.PersistMessages(context.Background(), "sess-buffer", messages))

	var count int64
	require.NoError(t, db.Model(&models.ConnectionSessionMessage{}).Where("session_id = ?", "sess-buffer").Count(&count).Error)
	require.EqualValues(t, 2, count)

	listed, err := chatSvc.ListMessages(context.Background(), "sess-buffer", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, "first", listed[0].Content)
	require.Equal(t, "second", listed[1].Content)
}

func TestSessionChatService_PostMessageLengthGuard(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	active := NewActiveSessionService(nil)
	lifecycle, err := NewSessionLifecycleService(
		db,
		active,
		WithSessionAuditService(auditSvc),
	)
	require.NoError(t, err)

	createChatFixtures(t, db, "conn-1", "user-1")

	session, err := lifecycle.StartSession(context.Background(), StartSessionParams{
		SessionID:     "sess-chat-2",
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

	chatSvc, err := NewSessionChatService(db, active)
	require.NoError(t, err)

	long := strings.Repeat("a", maxChatMessageLength+1)
	_, err = chatSvc.PostMessage(context.Background(), ChatMessageParams{
		SessionID: session.ID,
		AuthorID:  "user-1",
		Content:   long,
	})
	require.Error(t, err)
}

func TestSessionChatService_PostMessageInactiveSession(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	active := NewActiveSessionService(nil)
	chatSvc, err := NewSessionChatService(db, active)
	require.NoError(t, err)

	createChatFixtures(t, db, "conn-1", "user-1")
	require.NoError(t, db.Create(&models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-offline"},
		ConnectionID:    "conn-1",
		ProtocolID:      "ssh",
		OwnerUserID:     "user-1",
		Status:          SessionStatusClosed,
		StartedAt:       time.Now().Add(-time.Hour),
		LastHeartbeatAt: time.Now().Add(-30 * time.Minute),
		ClosedAt:        timePtr(time.Now().Add(-30 * time.Minute)),
	}).Error)

	message, err := chatSvc.PostMessage(context.Background(), ChatMessageParams{
		SessionID: "sess-offline",
		AuthorID:  "user-1",
		Content:   "stored",
	})
	require.NoError(t, err)
	require.Equal(t, "stored", message.Content)

	var stored models.ConnectionSessionMessage
	require.NoError(t, db.First(&stored, "session_id = ? AND author_id = ?", "sess-offline", "user-1").Error)
	require.Equal(t, "stored", stored.Content)
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func createChatFixtures(t *testing.T, db *gorm.DB, connectionID, ownerID string) {
	t.Helper()
	owner := models.User{
		BaseModel: models.BaseModel{ID: ownerID},
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&owner).Error)

	conn := models.Connection{
		BaseModel:   models.BaseModel{ID: connectionID},
		Name:        "Primary SSH",
		ProtocolID:  "ssh",
		OwnerUserID: ownerID,
	}
	require.NoError(t, db.Create(&conn).Error)
}
