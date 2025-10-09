package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/notifications"
)

func TestNotificationServiceCreateAndList(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-123"},
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	hub := notifications.NewHub()
	svc, err := NewNotificationService(db, hub)
	require.NoError(t, err)

	ctx := context.Background()
	dto, err := svc.Create(ctx, CreateNotificationInput{
		UserID:   user.ID,
		Type:     "connection.failed",
		Title:    "Connection failed",
		Message:  "SSH production cluster unreachable",
		Severity: "warning",
		Metadata: map[string]any{"connection_id": "conn-1"},
	})
	require.NoError(t, err)
	require.Equal(t, "connection.failed", dto.Type)

	items, err := svc.ListForUser(ctx, ListNotificationsInput{UserID: user.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, dto.ID, items[0].ID)
	require.False(t, items[0].IsRead)
}

func TestNotificationServiceMarkReadAndUnread(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-1"},
		Username:  "bob",
		Email:     "bob@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	svc, err := NewNotificationService(db, notifications.NewHub())
	require.NoError(t, err)

	ctx := context.Background()
	dto, err := svc.Create(ctx, CreateNotificationInput{
		UserID:  user.ID,
		Type:    "session.share",
		Title:   "Session shared",
		Message: "A session was shared with you",
	})
	require.NoError(t, err)

	read, err := svc.MarkRead(ctx, user.ID, dto.ID)
	require.NoError(t, err)
	require.True(t, read.IsRead)
	require.NotNil(t, read.ReadAt)

	unread, err := svc.MarkUnread(ctx, user.ID, dto.ID)
	require.NoError(t, err)
	require.False(t, unread.IsRead)
	require.Nil(t, unread.ReadAt)
}

func TestNotificationServiceDeleteAndMarkAll(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-xyz"},
		Username:  "charlie",
		Email:     "charlie@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	svc, err := NewNotificationService(db, notifications.NewHub())
	require.NoError(t, err)

	ctx := context.Background()
	first, err := svc.Create(ctx, CreateNotificationInput{
		UserID:  user.ID,
		Type:    "system.update",
		Title:   "System Updated",
		Message: "ShellCN was upgraded",
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, CreateNotificationInput{
		UserID:  user.ID,
		Type:    "session.ended",
		Title:   "Session ended",
		Message: "Your SSH session ended",
	})
	require.NoError(t, err)

	require.NoError(t, svc.MarkAllRead(ctx, user.ID))

	items, err := svc.ListForUser(ctx, ListNotificationsInput{UserID: user.ID})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.True(t, item.IsRead)
	}

	require.NoError(t, svc.Delete(ctx, user.ID, first.ID))

	items, err = svc.ListForUser(ctx, ListNotificationsInput{UserID: user.ID})
	require.NoError(t, err)
	require.Len(t, items, 1)
}
