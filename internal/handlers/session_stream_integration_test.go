package handlers

import (
	"context"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestRealtimeSessionStream_PropagatesLifecycleEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := realtime.NewHub()
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "integration-secret"})
	require.NoError(t, err)

	handler := NewRealtimeHandler(hub, jwtSvc, nil, realtime.StreamConnectionSessions)
	router := gin.New()
	router.GET("/ws", handler.Stream)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	active := services.NewActiveSessionService(hub)
	lifecycle, err := services.NewSessionLifecycleService(
		db,
		active,
		services.WithLifecycleClock(func() time.Time { return time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC) }),
	)
	require.NoError(t, err)

	owner := models.User{
		BaseModel: models.BaseModel{ID: "owner-1"},
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&owner).Error)

	connection := models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-1"},
		Name:        "Primary SSH",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	token, err := jwtSvc.GenerateAccessToken(iauth.AccessTokenInput{UserID: owner.ID})
	require.NoError(t, err)

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"
	query := neturl.Values{}
	query.Set("streams", realtime.StreamConnectionSessions)
	query.Set("token", token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	session, err := lifecycle.StartSession(context.Background(), services.StartSessionParams{
		SessionID:      "sess-live",
		ConnectionID:   connection.ID,
		ConnectionName: connection.Name,
		ProtocolID:     "ssh",
		OwnerUserID:    owner.ID,
		OwnerUserName:  owner.Username,
		Actor: services.SessionActor{
			UserID:   owner.ID,
			Username: owner.Username,
		},
	})
	require.NoError(t, err)

	msg := readRealtimeMessage(t, conn)
	require.Equal(t, realtime.StreamConnectionSessions, msg.Stream)
	require.Equal(t, "session.opened", msg.Event)
	payload := messageData(t, msg.Data)
	require.Equal(t, session.ID, payload["id"])
	require.Equal(t, connection.ID, payload["connection_id"])

	participant := models.User{
		BaseModel: models.BaseModel{ID: "user-2"},
		Username:  "bob",
		Email:     "bob@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&participant).Error)

	_, err = active.AddParticipant(session.ID, services.ActiveSessionParticipant{
		UserID:   participant.ID,
		UserName: participant.Username,
	})
	require.NoError(t, err)

	msg = readRealtimeMessage(t, conn)
	require.Equal(t, "session.participant_joined", msg.Event)
	payload = messageData(t, msg.Data)
	require.Equal(t, participant.ID, payload["user_id"])
	mode, ok := payload["access_mode"].(string)
	require.True(t, ok)
	require.Equal(t, "read", strings.ToLower(mode))

	_, err = active.GrantWriteAccess(session.ID, participant.ID)
	require.NoError(t, err)

	msg = readRealtimeMessage(t, conn)
	require.Equal(t, "session.write_granted", msg.Event)
	payload = messageData(t, msg.Data)
	require.Equal(t, participant.ID, payload["user_id"])

	_, err = active.AppendChatMessage(session.ID, services.ActiveSessionChatMessage{
		AuthorID: owner.ID,
		Author:   owner.Username,
		Content:  "hello team",
	})
	require.NoError(t, err)

	msg = readRealtimeMessage(t, conn)
	require.Equal(t, "session.chat_posted", msg.Event)
	payload = messageData(t, msg.Data)
	content, ok := payload["content"].(string)
	require.True(t, ok)
	require.Equal(t, "hello team", content)
	authorID, ok := payload["author_id"].(string)
	require.True(t, ok)
	require.Equal(t, owner.ID, authorID)
}

func readRealtimeMessage(t *testing.T, conn *websocket.Conn) realtime.Message {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	var msg realtime.Message
	require.NoError(t, conn.ReadJSON(&msg))
	return msg
}

func messageData(t *testing.T, raw any) map[string]any {
	t.Helper()
	data, ok := raw.(map[string]any)
	require.True(t, ok, "expected map payload")
	return data
}
