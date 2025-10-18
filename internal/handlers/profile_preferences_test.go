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
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestProfileHandler_PreferencesLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	userSvc, err := services.NewUserService(db, auditSvc)
	require.NoError(t, err)

	user, err := userSvc.Create(context.Background(), services.CreateUserInput{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	prefSvc, err := services.NewUserPreferencesService(db, auditSvc)
	require.NoError(t, err)

	handler := NewProfileHandler(userSvc, prefSvc, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, user.ID)
		ctx := auditctx.WithActor(c.Request.Context(), auditctx.Actor{
			UserID:   user.ID,
			Username: user.Username,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	profile := router.Group("/api/profile")
	{
		profile.GET("/preferences", handler.GetPreferences)
		profile.PUT("/preferences", handler.UpdatePreferences)
	}

	getReq, _ := http.NewRequest(http.MethodGet, "/api/profile/preferences", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getEnv apiEnvelope
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getEnv))
	require.True(t, getEnv.Success)

	var initial services.UserPreferences
	require.NoError(t, json.Unmarshal(getEnv.Data, &initial))
	require.Equal(t, services.DefaultUserPreferences(), initial)

	payload := map[string]any{
		"ssh": map[string]any{
			"terminal": map[string]any{
				"font_family":    "JetBrains Mono",
				"cursor_style":   "underline",
				"copy_on_select": false,
			},
			"sftp": map[string]any{
				"show_hidden_files": true,
				"auto_open_queue":   false,
			},
		},
	}
	body, _ := json.Marshal(payload)

	putReq, _ := http.NewRequest(http.MethodPut, "/api/profile/preferences", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	require.Equal(t, http.StatusOK, putRec.Code)

	var putEnv apiEnvelope
	require.NoError(t, json.Unmarshal(putRec.Body.Bytes(), &putEnv))
	require.True(t, putEnv.Success)

	var updated services.UserPreferences
	require.NoError(t, json.Unmarshal(putEnv.Data, &updated))
	require.Equal(t, "JetBrains Mono", updated.SSH.Terminal.FontFamily)
	require.Equal(t, "underline", updated.SSH.Terminal.CursorStyle)
	require.False(t, updated.SSH.Terminal.CopyOnSelect)
	require.True(t, updated.SSH.SFTP.ShowHiddenFiles)
	require.False(t, updated.SSH.SFTP.AutoOpenQueue)
}
