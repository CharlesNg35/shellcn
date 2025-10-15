package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestSessionRecordingHandler_StatusStopDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	activeSvc := services.NewActiveSessionService(nil)
	sessionChatSvc, err := services.NewSessionChatService(db, activeSvc)
	require.NoError(t, err)

	storeRoot := t.TempDir()
	recorderStore, err := services.NewFilesystemRecorderStore(storeRoot)
	require.NoError(t, err)
	recorderSvc, err := services.NewRecorderService(db, recorderStore, services.WithRecorderPolicy(services.RecorderPolicy{
		Mode:           services.RecordingModeForced,
		Storage:        "filesystem",
		RetentionDays:  0,
		RequireConsent: false,
	}))
	require.NoError(t, err)

	lifecycleSvc, err := services.NewSessionLifecycleService(
		db,
		activeSvc,
		services.WithSessionChatStore(sessionChatSvc),
		services.WithSessionRecorder(recorderSvc),
	)
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	handler := NewSessionRecordingHandler(recorderSvc, lifecycleSvc, checker)

	middlewareChain := func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			userID = ownerUserID
		}
		c.Set(middleware.CtxUserIDKey, userID)
		actor := auditctx.Actor{
			UserID:    userID,
			Username:  userID,
			IPAddress: "127.0.0.1",
			UserAgent: "test-suite",
		}
		ctx := auditctx.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

	router.GET("/api/active-sessions/:sessionID/recording/status", func(c *gin.Context) {
		c.Set("user_id", ownerUserID)
		middlewareChain(c)
	}, handler.Status)
	router.POST("/api/active-sessions/:sessionID/recording/stop", func(c *gin.Context) {
		c.Set("user_id", ownerUserID)
		middlewareChain(c)
	}, handler.Stop)
	router.GET("/api/session-records/:recordID/download", func(c *gin.Context) {
		c.Set("user_id", ownerUserID)
		middlewareChain(c)
	}, handler.Download)

	owner := createRecordingTestUser(t, db, ownerUserID)
	connection := createRecordingTestConnection(t, db, owner.ID)

	startCtx := contextWithActor(owner.ID, owner.Username)

	session, err := lifecycleSvc.StartSession(startCtx, services.StartSessionParams{
		SessionID:      "rec-session",
		ConnectionID:   connection.ID,
		ConnectionName: connection.Name,
		ProtocolID:     "ssh",
		OwnerUserID:    owner.ID,
		OwnerUserName:  owner.Username,
		Metadata: map[string]any{
			"recording_enabled": true,
			"terminal_width":    100,
			"terminal_height":   40,
		},
	})
	require.NoError(t, err)

	recorderSvc.RecordStream(session.ID, "stdout", []byte("hello world\n"))

	// Status while active
	statusReq, _ := http.NewRequest(http.MethodGet, "/api/active-sessions/"+session.ID+"/recording/status", nil)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code, statusRec.Body.String())

	var envelope apiEnvelope
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)

	var statusPayload map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &statusPayload))
	require.Equal(t, true, statusPayload["active"])
	require.Equal(t, "forced", statusPayload["recording_mode"])

	// Stop recording
	stopReq, _ := http.NewRequest(http.MethodPost, "/api/active-sessions/"+session.ID+"/recording/stop", nil)
	stopRec := httptest.NewRecorder()
	router.ServeHTTP(stopRec, stopReq)
	require.Equal(t, http.StatusOK, stopRec.Code, stopRec.Body.String())
	require.NoError(t, json.Unmarshal(stopRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)

	var recordDTO recordingRecordDTO
	require.NoError(t, json.Unmarshal(envelope.Data, &recordDTO))
	require.Equal(t, session.ID, recordDTO.SessionID)
	require.NotEmpty(t, recordDTO.ID)
	require.Greater(t, recordDTO.SizeBytes, int64(0))
	require.NotNil(t, recordDTO.CreatedAt)
	require.Nil(t, recordDTO.RetentionUntil)

	// Status after stop
	statusRec = httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code)
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.NoError(t, json.Unmarshal(envelope.Data, &statusPayload))
	require.Equal(t, false, statusPayload["active"])
	require.Equal(t, "forced", statusPayload["recording_mode"])

	// Download recording
	downloadReq, _ := http.NewRequest(http.MethodGet, "/api/session-records/"+recordDTO.ID+"/download", nil)
	downloadRec := httptest.NewRecorder()
	router.ServeHTTP(downloadRec, downloadReq)
	require.Equal(t, http.StatusOK, downloadRec.Code, downloadRec.Body.String())
	require.Equal(t, "application/gzip", downloadRec.Header().Get("Content-Type"))

	gzr, err := gzip.NewReader(bytes.NewReader(downloadRec.Body.Bytes()))
	require.NoError(t, err)
	defer gzr.Close()
	content, err := io.ReadAll(gzr)
	require.NoError(t, err)
	require.Contains(t, string(content), "hello world")
}

const ownerUserID = "owner-recording"

func createRecordingTestUser(t *testing.T, db *gorm.DB, userID string) *models.User {
	t.Helper()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Username:  userID,
		Email:     userID + "@example.com",
		Password:  "password",
		IsActive:  true,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func createRecordingTestConnection(t *testing.T, db *gorm.DB, ownerID string) *models.Connection {
	t.Helper()
	conn := &models.Connection{
		Name:        "Recording Connection",
		ProtocolID:  "ssh",
		OwnerUserID: ownerID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(conn).Error)
	return conn
}
