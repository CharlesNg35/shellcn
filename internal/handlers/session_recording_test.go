package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	"github.com/charlesng35/shellcn/pkg/response"
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

func TestSessionRecordingHandler_ListAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	require.NoError(t, permissions.Sync(context.Background(), db))

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

	router.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-Test-User")
		if strings.TrimSpace(userID) != "" {
			c.Set(middleware.CtxUserIDKey, userID)
			actor := auditctx.Actor{
				UserID:    userID,
				Username:  userID,
				IPAddress: "127.0.0.1",
				UserAgent: "test-suite",
			}
			ctx := auditctx.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})

	api := router.Group("/api")
	api.GET("/session-records", middleware.RequirePermission(checker, "session.recording.view"), handler.List)
	api.DELETE("/session-records/:recordID", middleware.RequirePermission(checker, "session.recording.delete"), handler.Delete)
	api.GET("/session-records/:recordID/download", middleware.RequirePermission(checker, "session.recording.view"), handler.Download)

	var (
		permConnectionView models.Permission
		permRecordView     models.Permission
		permRecordTeam     models.Permission
		permRecordAll      models.Permission
		permRecordDelete   models.Permission
	)
	require.NoError(t, db.First(&permConnectionView, "id = ?", "connection.view").Error)
	require.NoError(t, db.First(&permRecordView, "id = ?", "session.recording.view").Error)
	require.NoError(t, db.First(&permRecordTeam, "id = ?", "session.recording.view_team").Error)
	require.NoError(t, db.First(&permRecordAll, "id = ?", "session.recording.view_all").Error)
	require.NoError(t, db.First(&permRecordDelete, "id = ?", "session.recording.delete").Error)

	roleViewer := &models.Role{
		BaseModel: models.BaseModel{ID: "role.record.viewer"},
		Name:      "Recording Viewer",
	}
	roleManager := &models.Role{
		BaseModel: models.BaseModel{ID: "role.record.manager"},
		Name:      "Recording Manager",
	}
	require.NoError(t, db.Create(roleViewer).Error)
	require.NoError(t, db.Create(roleManager).Error)
	require.NoError(t, db.Model(roleViewer).
		Association("Permissions").
		Append(&permConnectionView, &permRecordView, &permRecordTeam))
	require.NoError(t, db.Model(roleManager).
		Association("Permissions").
		Append(&permConnectionView, &permRecordAll, &permRecordDelete))

	viewer := &models.User{
		BaseModel: models.BaseModel{ID: "user-viewer"},
		Username:  "viewer",
		Email:     "viewer@example.com",
		Password:  "password",
		IsActive:  true,
	}
	manager := &models.User{
		BaseModel: models.BaseModel{ID: "user-manager"},
		Username:  "manager",
		Email:     "manager@example.com",
		Password:  "password",
		IsActive:  true,
	}
	otherUser := &models.User{
		BaseModel: models.BaseModel{ID: "user-other"},
		Username:  "other",
		Email:     "other@example.com",
		Password:  "password",
		IsActive:  true,
	}
	require.NoError(t, db.Create(viewer).Error)
	require.NoError(t, db.Create(manager).Error)
	require.NoError(t, db.Create(otherUser).Error)
	require.NoError(t, db.Model(viewer).Association("Roles").Append(roleViewer))
	require.NoError(t, db.Model(manager).Association("Roles").Append(roleManager))

	team := &models.Team{
		BaseModel: models.BaseModel{ID: "team-dev"},
		Name:      "DevOps",
	}
	otherTeam := &models.Team{
		BaseModel: models.BaseModel{ID: "team-sec"},
		Name:      "Security",
	}
	require.NoError(t, db.Create(team).Error)
	require.NoError(t, db.Create(otherTeam).Error)
	require.NoError(t, db.Model(team).Association("Users").Append(viewer))
	require.NoError(t, db.Model(otherTeam).Association("Users").Append(otherUser))

	now := time.Now().UTC()

	connTeam := &models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-team"},
		Name:        "Team Connection",
		ProtocolID:  "ssh",
		OwnerUserID: viewer.ID,
		TeamID:      &team.ID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(connTeam).Error)

	connPersonal := &models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-personal"},
		Name:        "Personal Connection",
		ProtocolID:  "ssh",
		OwnerUserID: viewer.ID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(connPersonal).Error)

	connOther := &models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-other"},
		Name:        "Other Connection",
		ProtocolID:  "ssh",
		OwnerUserID: otherUser.ID,
		TeamID:      &otherTeam.ID,
		Settings:    datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(connOther).Error)

	sessionTeam := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-team"},
		ConnectionID:    connTeam.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     viewer.ID,
		TeamID:          &team.ID,
		Status:          services.SessionStatusClosed,
		StartedAt:       now.Add(-2 * time.Hour),
		LastHeartbeatAt: now.Add(-2 * time.Hour),
	}
	sessionPersonal := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-personal"},
		ConnectionID:    connPersonal.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     viewer.ID,
		Status:          services.SessionStatusClosed,
		StartedAt:       now.Add(-time.Hour),
		LastHeartbeatAt: now.Add(-time.Hour),
	}
	sessionOther := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: "sess-other"},
		ConnectionID:    connOther.ID,
		ProtocolID:      "ssh",
		OwnerUserID:     otherUser.ID,
		TeamID:          &otherTeam.ID,
		Status:          services.SessionStatusClosed,
		StartedAt:       now.Add(-90 * time.Minute),
		LastHeartbeatAt: now.Add(-90 * time.Minute),
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
			DurationSeconds: 80,
			CreatedByUserID: viewer.ID,
		},
		{
			BaseModel:       models.BaseModel{ID: "rec-personal"},
			SessionID:       sessionPersonal.ID,
			StorageKind:     "filesystem",
			StoragePath:     "personal.cast.gz",
			SizeBytes:       200,
			DurationSeconds: 120,
			CreatedByUserID: viewer.ID,
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
		fullPath := filepath.Join(storeRoot, record.StoragePath)
		require.NoError(t, os.WriteFile(fullPath, []byte(record.ID), 0o600))
	}

	doRequest := func(method, path, userID string) *httptest.ResponseRecorder {
		req, err := http.NewRequest(method, path, nil)
		require.NoError(t, err)
		if userID != "" {
			req.Header.Set("X-Test-User", userID)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	parseList := func(body []byte) ([]map[string]any, response.Meta) {
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(body, &envelope))
		require.True(t, envelope.Success)
		var items []map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &items))
		meta := response.Meta{}
		if len(envelope.Meta) > 0 {
			require.NoError(t, json.Unmarshal(envelope.Meta, &meta))
		}
		return items, meta
	}

	t.Run("viewer personal scope", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=personal", viewer.ID)
		require.Equalf(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		items, meta := parseList(rec.Body.Bytes())
		require.Len(t, items, 2)
		require.Equal(t, 2, meta.Total)
	})

	t.Run("viewer team scope", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=team", viewer.ID)
		require.Equalf(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		items, _ := parseList(rec.Body.Bytes())
		require.Len(t, items, 2)
	})

	t.Run("viewer team scope filtered", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=team&team_id="+team.ID, viewer.ID)
		require.Equalf(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		items, _ := parseList(rec.Body.Bytes())
		require.Len(t, items, 1)
		require.Equal(t, "rec-team", items[0]["record_id"])
	})

	t.Run("viewer all scope forbidden", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=all", viewer.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("manager all scope", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=all", manager.ID)
		require.Equalf(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		items, meta := parseList(rec.Body.Bytes())
		require.Len(t, items, 3)
		require.Equal(t, 3, meta.Total)
	})

	t.Run("manager all scope filtered to other team", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records?scope=all&team_id="+otherTeam.ID, manager.ID)
		require.Equalf(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		items, _ := parseList(rec.Body.Bytes())
		require.Len(t, items, 1)
		require.Equal(t, "rec-other", items[0]["record_id"])
	})

	t.Run("viewer delete forbidden", func(t *testing.T) {
		rec := doRequest(http.MethodDelete, "/api/session-records/rec-other", viewer.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("manager delete succeeds", func(t *testing.T) {
		rec := doRequest(http.MethodDelete, "/api/session-records/rec-other", manager.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)

		var count int64
		require.NoError(t, db.Model(&models.ConnectionSessionRecord{}).
			Where("id = ?", "rec-other").
			Count(&count).Error)
		require.Zero(t, count)
		_, statErr := os.Stat(filepath.Join(storeRoot, "other.cast.gz"))
		require.True(t, errors.Is(statErr, os.ErrNotExist))
	})

	t.Run("manager download succeeds", func(t *testing.T) {
		rec := doRequest(http.MethodGet, "/api/session-records/rec-team/download", manager.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/gzip", rec.Header().Get("Content-Type"))
	})
}
