package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestSessionChatHandler_PostAndListMessages(t *testing.T) {
	g := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	active := services.NewActiveSessionService(nil)
	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	chatSvc, err := services.NewSessionChatService(db, active)
	require.NoError(t, err)
	lifecycleSvc, err := services.NewSessionLifecycleService(db, active, services.WithSessionAuditService(auditSvc))
	require.NoError(t, err)

	owner := createTestUser(t, db, "owner-user")
	connection := createTestConnection(t, db, owner.ID)

	_, err = lifecycleSvc.StartSession(
		contextWithActor(owner.ID, owner.Username),
		services.StartSessionParams{
			SessionID:      "sess-http",
			ConnectionID:   connection.ID,
			ConnectionName: connection.Name,
			ProtocolID:     "ssh",
			OwnerUserID:    owner.ID,
			OwnerUserName:  owner.Username,
		},
	)
	require.NoError(t, err)

	handler := NewSessionChatHandler(chatSvc, lifecycleSvc)
	mw := func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, owner.ID)
		ctx := auditctx.WithActor(c.Request.Context(), auditctx.Actor{
			UserID:    owner.ID,
			Username:  owner.Username,
			IPAddress: "127.0.0.1",
			UserAgent: "test-suite",
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

	g.POST("/api/active-sessions/:sessionID/chat", mw, handler.PostMessage)
	g.GET("/api/active-sessions/:sessionID/chat", mw, handler.ListMessages)

	postBody, _ := json.Marshal(map[string]string{"content": "hello world"})
	postReq, _ := http.NewRequest(http.MethodPost, "/api/active-sessions/sess-http/chat", bytes.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	g.ServeHTTP(postRec, postReq)
	require.Equal(t, http.StatusCreated, postRec.Code, postRec.Body.String())

	var postResp apiEnvelope
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &postResp))
	require.True(t, postResp.Success)
	var created chatMessageDTO
	require.NoError(t, json.Unmarshal(postResp.Data, &created))
	require.Equal(t, "hello world", created.Content)
	require.Equal(t, owner.ID, created.AuthorID)

	getReq, _ := http.NewRequest(http.MethodGet, "/api/active-sessions/sess-http/chat?limit=10", nil)
	getRec := httptest.NewRecorder()
	g.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())

	var listResp apiEnvelope
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &listResp))
	require.True(t, listResp.Success)
	var messages []chatMessageDTO
	require.NoError(t, json.Unmarshal(listResp.Data, &messages))
	require.Len(t, messages, 1)
	require.Equal(t, "hello world", messages[0].Content)
	require.WithinDuration(t, time.Now(), messages[0].CreatedAt, 2*time.Second)
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func createTestUser(t *testing.T, db *gorm.DB, username string) *models.User {
	t.Helper()
	user := &models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: "password",
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func createTestConnection(t *testing.T, db *gorm.DB, ownerID string) *models.Connection {
	t.Helper()
	conn := &models.Connection{
		Name:        "SSH Test",
		ProtocolID:  "ssh",
		OwnerUserID: ownerID,
	}
	require.NoError(t, db.Create(conn).Error)
	return conn
}

func contextWithActor(userID, username string) context.Context {
	return auditctx.WithActor(context.Background(), auditctx.Actor{
		UserID:    userID,
		Username:  username,
		IPAddress: "127.0.0.1",
		UserAgent: "test-suite",
	})
}
