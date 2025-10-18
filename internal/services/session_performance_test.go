package services

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestSessionLifecyclePerformanceSmoke(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	active := NewActiveSessionService(nil)
	lifecycle, err := NewSessionLifecycleService(
		db,
		active,
		WithLifecycleClock(func() time.Time { return time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC) }),
	)
	require.NoError(t, err)

	owner := models.User{
		BaseModel: models.BaseModel{ID: "perf-owner"},
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&owner).Error)

	connection := models.Connection{
		BaseModel:   models.BaseModel{ID: "perf-conn"},
		Name:        "SSH Perf",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	const iterations = 25

	runtime.GC()
	start := time.Now()

	for i := 0; i < iterations; i++ {
		sessionID := fmt.Sprintf("perf-%d", i)
		_, err := lifecycle.StartSession(context.Background(), StartSessionParams{
			SessionID:      sessionID,
			ConnectionID:   connection.ID,
			ConnectionName: connection.Name,
			ProtocolID:     "ssh",
			OwnerUserID:    owner.ID,
			OwnerUserName:  owner.Username,
			Actor: SessionActor{
				UserID:   owner.ID,
				Username: owner.Username,
			},
		})
		require.NoError(t, err)

		require.NoError(t, lifecycle.Heartbeat(context.Background(), sessionID))

		_, err = active.AppendChatMessage(sessionID, ActiveSessionChatMessage{
			AuthorID: owner.ID,
			Author:   owner.Username,
			Content:  "load-test",
		})
		require.NoError(t, err)

		require.NoError(t, lifecycle.CloseSession(context.Background(), CloseSessionParams{
			SessionID: sessionID,
			Status:    SessionStatusClosed,
			Actor: SessionActor{
				UserID:   owner.ID,
				Username: owner.Username,
			},
		}))
	}

	elapsed := time.Since(start)
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	avgLatency := elapsed / iterations
	throughput := float64(iterations) / elapsed.Seconds()
	t.Logf("ssh session smoke: iterations=%d avg=%s throughput=%.2f/s heap_alloc=%d bytes", iterations, avgLatency, throughput, stats.Alloc)
}
