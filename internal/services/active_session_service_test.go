package services

import (
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

func ptrString(value string) *string {
	return &value
}
