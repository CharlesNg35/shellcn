package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/database"
	testutil "github.com/charlesng35/shellcn/internal/database/testutil"
	_ "github.com/charlesng35/shellcn/internal/drivers/ssh"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestProtocolSettingsHandler_GetAndUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())

	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := services.NewUserService(db, auditSvc)
	require.NoError(t, err)

	rootUser, err := userSvc.Create(context.Background(), services.CreateUserInput{
		Username: "root",
		Email:    "root@example.com",
		Password: "password",
		IsRoot:   true,
	})
	require.NoError(t, err)

	store, err := services.NewFilesystemRecorderStore(t.TempDir())
	require.NoError(t, err)

	recorder, err := services.NewRecorderService(db, store)
	require.NoError(t, err)

	svc, err := services.NewProtocolSettingsService(db, auditSvc, services.WithProtocolRecorder(recorder))
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	handler := NewProtocolSettingsHandler(svc, checker)

	allowed, authErr := handler.authorize(context.Background(), "user-1")
	t.Logf("authorise: allowed=%v err=%v", allowed, authErr)

	router.Use(func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, rootUser.ID)
		ctx := auditctx.WithActor(c.Request.Context(), auditctx.Actor{UserID: rootUser.ID, Username: "root"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	router.GET("/api/settings/protocols/ssh", handler.GetSSHSettings)
	router.PUT("/api/settings/protocols/ssh", handler.UpdateSSHSettings)

	// GET defaults
	getRec := httptest.NewRecorder()
	getReq, _ := http.NewRequest(http.MethodGet, "/api/settings/protocols/ssh", nil)
	router.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getEnvelope apiEnvelope
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getEnvelope))
	require.True(t, getEnvelope.Success)

	var getPayload services.SSHProtocolSettings
	require.NoError(t, json.Unmarshal(getEnvelope.Data, &getPayload))
	require.Equal(t, services.RecordingModeOptional, getPayload.Recording.Mode)
	require.Equal(t, 0, getPayload.Session.ConcurrentLimit)
	require.True(t, getPayload.Session.EnableSFTP)

	// Update recording defaults
	body := map[string]any{
		"session": map[string]any{
			"concurrent_limit":     3,
			"idle_timeout_minutes": 45,
			"enable_sftp":          true,
		},
		"terminal": map[string]any{
			"theme_mode":       "force_light",
			"font_family":      "Fira Code",
			"font_size":        14,
			"scrollback_limit": 2000,
		},
		"recording": map[string]any{
			"mode":            services.RecordingModeForced,
			"storage":         "filesystem",
			"retention_days":  15,
			"require_consent": false,
		},
		"collaboration": map[string]any{
			"allow_sharing":            true,
			"restrict_write_to_admins": false,
		},
	}
	payloadBytes, _ := json.Marshal(body)
	putRec := httptest.NewRecorder()
	putReq, _ := http.NewRequest(http.MethodPut, "/api/settings/protocols/ssh", bytes.NewReader(payloadBytes))
	putReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(putRec, putReq)
	require.Equal(t, http.StatusOK, putRec.Code)

	var putEnvelope apiEnvelope
	require.NoError(t, json.Unmarshal(putRec.Body.Bytes(), &putEnvelope))
	require.True(t, putEnvelope.Success)

	var updated services.SSHProtocolSettings
	require.NoError(t, json.Unmarshal(putEnvelope.Data, &updated))
	require.Equal(t, services.RecordingModeForced, updated.Recording.Mode)
	require.Equal(t, 15, updated.Recording.RetentionDays)
	require.Equal(t, 3, updated.Session.ConcurrentLimit)
	require.Equal(t, "Fira Code", updated.Terminal.FontFamily)

	modeValue, err := database.GetSystemSetting(context.Background(), db, "recording.mode")
	require.NoError(t, err)
	require.Equal(t, services.RecordingModeForced, modeValue)
}

func TestProtocolSettingsHandler_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())

	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := services.NewUserService(db, auditSvc)
	require.NoError(t, err)

	user, err := userSvc.Create(context.Background(), services.CreateUserInput{
		Username: "user",
		Email:    "user@example.com",
		Password: "password",
	})
	require.NoError(t, err)

	svc, err := services.NewProtocolSettingsService(db, auditSvc)
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	handler := NewProtocolSettingsHandler(svc, checker)

	allowed, authErr := handler.authorize(context.Background(), user.ID)
	t.Logf("authorize preflight: allowed=%v err=%v", allowed, authErr)

	router.Use(func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, user.ID)
		ctx := auditctx.WithActor(c.Request.Context(), auditctx.Actor{UserID: user.ID})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	router.PUT("/api/settings/protocols/ssh", handler.UpdateSSHSettings)

	body := map[string]any{
		"session": map[string]any{
			"concurrent_limit":     1,
			"idle_timeout_minutes": 30,
			"enable_sftp":          true,
		},
		"terminal": map[string]any{
			"theme_mode":       "auto",
			"font_family":      "monospace",
			"font_size":        14,
			"scrollback_limit": 1000,
		},
		"recording": map[string]any{
			"mode":            services.RecordingModeForced,
			"storage":         "filesystem",
			"retention_days":  15,
			"require_consent": false,
		},
		"collaboration": map[string]any{
			"allow_sharing":            true,
			"restrict_write_to_admins": false,
		},
	}
	payloadBytes, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/settings/protocols/ssh", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	t.Log(rec.Body.String())

	require.Equal(t, http.StatusForbidden, rec.Code)

	var errEnvelope apiEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errEnvelope))
	require.False(t, errEnvelope.Success)
}
