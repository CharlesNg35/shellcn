package services

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActiveSessionService_RegisterAndList(t *testing.T) {
	svc := NewActiveSessionService(nil)
	t0 := time.Now().Add(-time.Minute)
	svc.timeNow = func() time.Time {
		return t0
	}

	session := &ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}

	err := svc.RegisterSession(session)
	require.NoError(t, err)
	require.Equal(t, 1, svc.Count())

	results := svc.ListActive(ListActiveOptions{UserID: "user-1"})
	require.Len(t, results, 1)
	require.Equal(t, "conn-1", results[0].ConnectionID)
	require.Equal(t, "alice", results[0].UserName)
	require.False(t, results[0].StartedAt.IsZero())
	require.False(t, results[0].LastSeenAt.IsZero())
	require.Equal(t, "user-1", results[0].OwnerUserID)
	require.Equal(t, "alice", results[0].OwnerUserName)
	require.Equal(t, "user-1", results[0].WriteHolder)
	require.NotNil(t, results[0].Participants)
	require.Contains(t, results[0].Participants, "user-1")
	require.Equal(t, "write", results[0].Participants["user-1"].AccessMode)
}

func TestActiveSessionService_RegisterDuplicateUserConnection(t *testing.T) {
	svc := NewActiveSessionService(nil)
	now := time.Now()
	svc.timeNow = func() time.Time { return now }

	first := &ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}
	require.NoError(t, svc.RegisterSession(first))

	second := &ActiveSessionRecord{
		ID:           "sess-2",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}
	err := svc.RegisterSession(second)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrActiveSessionExists)
}

func TestActiveSessionService_HasActiveSession(t *testing.T) {
	svc := NewActiveSessionService(nil)
	now := time.Date(2024, 10, 1, 10, 0, 0, 0, time.UTC)
	svc.timeNow = func() time.Time { return now }

	require.False(t, svc.HasActiveSession("user-1", "conn-1"))

	session := &ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}
	require.NoError(t, svc.RegisterSession(session))
	require.True(t, svc.HasActiveSession("user-1", "conn-1"))

	svc.UnregisterSession("sess-1")
	require.False(t, svc.HasActiveSession("user-1", "conn-1"))
}

func TestActiveSessionService_UnregisterSession(t *testing.T) {
	svc := NewActiveSessionService(nil)

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}))

	svc.UnregisterSession("sess-1")
	require.Equal(t, 0, svc.Count())

	results := svc.ListActive(ListActiveOptions{UserID: "user-1"})
	require.Empty(t, results)
}

func TestActiveSessionService_ListActive_AdminSeesAll(t *testing.T) {
	svc := NewActiveSessionService(nil)
	now := time.Now()
	svc.timeNow = func() time.Time { return now }

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}))
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-2",
		ConnectionID: "conn-2",
		UserID:       "user-2",
		ProtocolID:   "ssh",
		TeamID:       ptrString("team-1"),
	}))

	userSessions := svc.ListActive(ListActiveOptions{UserID: "user-1"})
	require.Len(t, userSessions, 1)
	require.Equal(t, "conn-1", userSessions[0].ConnectionID)

	adminSessions := svc.ListActive(ListActiveOptions{IncludeAll: true})
	require.Len(t, adminSessions, 2)
}

func TestActiveSessionService_ListActive_TeamVisibility(t *testing.T) {
	svc := NewActiveSessionService(nil)

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
	}))
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-2",
		ConnectionID: "conn-2",
		UserID:       "user-2",
		ProtocolID:   "ssh",
		TeamID:       ptrString("team-1"),
	}))

	results := svc.ListActive(ListActiveOptions{
		UserID:       "user-3",
		IncludeTeams: true,
		TeamIDs:      []string{"team-1"},
	})
	require.Len(t, results, 1)
	require.Equal(t, "conn-2", results[0].ConnectionID)
}

func TestActiveSessionService_AddParticipantAndGrantWrite(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))

	added, err := svc.AddParticipant("sess-1", ActiveSessionParticipant{
		UserID:   "user-2",
		UserName: "bob",
	})
	require.NoError(t, err)
	require.Equal(t, "participant", added.Role)
	require.Equal(t, "read", added.AccessMode)

	session, ok := svc.GetSession("sess-1")
	require.True(t, ok)
	require.Contains(t, session.Participants, "user-2")
	require.Equal(t, "read", session.Participants["user-2"].AccessMode)
	require.Equal(t, "user-1", session.WriteHolder)

	granted, err := svc.GrantWriteAccess("sess-1", "user-2")
	require.NoError(t, err)
	require.Equal(t, "write", granted.AccessMode)

	session, ok = svc.GetSession("sess-1")
	require.True(t, ok)
	require.Equal(t, "user-2", session.WriteHolder)
	require.Equal(t, "read", session.Participants["user-1"].AccessMode)
	require.Equal(t, "write", session.Participants["user-2"].AccessMode)
}

func TestActiveSessionService_RemoveParticipantRevertsWrite(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))
	_, err := svc.AddParticipant("sess-1", ActiveSessionParticipant{
		UserID:     "user-2",
		UserName:   "bob",
		AccessMode: "write",
	})
	require.NoError(t, err)
	_, err = svc.GrantWriteAccess("sess-1", "user-2")
	require.NoError(t, err)

	removed := svc.RemoveParticipant("sess-1", "user-2")
	require.True(t, removed)

	session, ok := svc.GetSession("sess-1")
	require.True(t, ok)
	require.NotContains(t, session.Participants, "user-2")
	require.Equal(t, "user-1", session.WriteHolder)
	require.Equal(t, "write", session.Participants["user-1"].AccessMode)
}

func TestActiveSessionService_RelinquishWriteAccess(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "owner-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))

	_, err := svc.AddParticipant("sess-1", ActiveSessionParticipant{
		UserID:     "user-2",
		UserName:   "bob",
		AccessMode: "write",
	})
	require.NoError(t, err)
	_, err = svc.GrantWriteAccess("sess-1", "user-2")
	require.NoError(t, err)

	updated, newWriter, err := svc.RelinquishWriteAccess("sess-1", "user-2")
	require.NoError(t, err)
	require.Equal(t, "user-2", updated.UserID)
	require.Equal(t, "read", strings.ToLower(updated.AccessMode))
	require.NotNil(t, newWriter)
	require.Equal(t, "owner-1", newWriter.UserID)
	require.Equal(t, "write", strings.ToLower(newWriter.AccessMode))

	session, ok := svc.GetSession("sess-1")
	require.True(t, ok)
	require.Equal(t, "owner-1", session.WriteHolder)
	require.Equal(t, "read", session.Participants["user-2"].AccessMode)
	require.Equal(t, "write", session.Participants["owner-1"].AccessMode)

	updatedOwner, newWriter, err := svc.RelinquishWriteAccess("sess-1", "owner-1")
	require.NoError(t, err)
	require.Equal(t, "owner-1", updatedOwner.UserID)
	require.Equal(t, "read", strings.ToLower(updatedOwner.AccessMode))
	require.Nil(t, newWriter)

	session, ok = svc.GetSession("sess-1")
	require.True(t, ok)
	require.Equal(t, "", session.WriteHolder)
	require.Equal(t, "read", session.Participants["owner-1"].AccessMode)
}

func TestActiveSessionService_AppendAndConsumeChatBuffer(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))

	message, err := svc.AppendChatMessage("sess-1", ActiveSessionChatMessage{
		AuthorID: "user-1",
		Author:   "alice",
		Content:  "hello world",
	})
	require.NoError(t, err)
	require.NotEmpty(t, message.MessageID)

	buffer := svc.ConsumeChatBuffer("sess-1")
	require.Len(t, buffer, 1)
	require.Equal(t, "hello world", buffer[0].Content)

	buffer = svc.ConsumeChatBuffer("sess-1")
	require.Empty(t, buffer)
}

func TestActiveSessionService_AckChatMessage(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))

	message, err := svc.AppendChatMessage("sess-1", ActiveSessionChatMessage{
		AuthorID: "user-1",
		Content:  "pending",
	})
	require.NoError(t, err)

	require.True(t, svc.AckChatMessage("sess-1", message.MessageID))
	require.False(t, svc.AckChatMessage("sess-1", "missing"))

	buffer := svc.ConsumeChatBuffer("sess-1")
	require.Empty(t, buffer)
}

func TestActiveSessionService_CleanupStale(t *testing.T) {
	svc := NewActiveSessionService(nil)
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.timeNow = func() time.Time { return base }

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "user-1",
		ProtocolID:   "ssh",
		LastSeenAt:   base.Add(-10 * time.Minute),
		StartedAt:    base.Add(-15 * time.Minute),
	}))
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-2",
		ConnectionID: "conn-2",
		UserID:       "user-2",
		ProtocolID:   "ssh",
		LastSeenAt:   base.Add(-2 * time.Minute),
		StartedAt:    base.Add(-5 * time.Minute),
	}))

	svc.CleanupStale(5 * time.Minute)
	require.Equal(t, 1, svc.Count())

	results := svc.ListActive(ListActiveOptions{IncludeAll: true})
	require.Len(t, results, 1)
	require.Equal(t, "sess-2", results[0].ID)
}

func TestActiveSessionService_ConcurrentLimitEnforced(t *testing.T) {
	svc := NewActiveSessionService(nil)

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:              "sess-1",
		ConnectionID:    "conn-1",
		UserID:          "user-1",
		ProtocolID:      "ssh",
		ConcurrentLimit: 2,
	}))
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:              "sess-2",
		ConnectionID:    "conn-1",
		UserID:          "user-2",
		ProtocolID:      "ssh",
		ConcurrentLimit: 2,
	}))

	err := svc.RegisterSession(&ActiveSessionRecord{
		ID:              "sess-3",
		ConnectionID:    "conn-1",
		UserID:          "user-3",
		ProtocolID:      "ssh",
		ConcurrentLimit: 2,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrConcurrentLimitReached)
	var limitErr *ConcurrentLimitError
	require.ErrorAs(t, err, &limitErr)
	require.NotNil(t, limitErr)
	require.Equal(t, "conn-1", limitErr.ConnectionID)
	require.Equal(t, 2, limitErr.Limit)
	require.Equal(t, ConcurrentLimitReasonReached, limitErr.Reason)

	svc.UnregisterSession("sess-2")

	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:              "sess-4",
		ConnectionID:    "conn-1",
		UserID:          "user-4",
		ProtocolID:      "ssh",
		ConcurrentLimit: 2,
	}))
}

func TestActiveSessionService_GrantWriteAccessMissingParticipant(t *testing.T) {
	svc := NewActiveSessionService(nil)
	require.NoError(t, svc.RegisterSession(&ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       "owner-1",
		UserName:     "alice",
		ProtocolID:   "ssh",
	}))

	_, err := svc.GrantWriteAccess("sess-1", "missing-user")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "participant missing-user not found")

	session, ok := svc.GetSession("sess-1")
	require.True(t, ok)
	require.Equal(t, "owner-1", session.WriteHolder)
	require.Equal(t, "write", session.Participants["owner-1"].AccessMode)
}

func ptrString(value string) *string {
	return &value
}
