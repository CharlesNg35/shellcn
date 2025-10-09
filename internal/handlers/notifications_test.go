package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/notifications"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/response"
)

func TestNotificationHandlerListAndMarkRead(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	hub := notifications.NewHub()
	handler, err := NewNotificationHandler(db, hub, nil)
	require.NoError(t, err)

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-handler"},
		Username:  "dana",
		Email:     "dana@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	_, err = handler.service.Create(testContext(), services.CreateNotificationInput{
		UserID:  user.ID,
		Type:    "session.shared",
		Title:   "Session shared",
		Message: "A teammate shared a session",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.CtxUserIDKey, user.ID)
	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	dataBytes, err := json.Marshal(payload.Data)
	require.NoError(t, err)

	var notificationsDTO []services.NotificationDTO
	require.NoError(t, json.Unmarshal(dataBytes, &notificationsDTO))
	require.Len(t, notificationsDTO, 1)

	notificationID := notificationsDTO[0].ID

	readRecorder := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(readRecorder)
	c2.Params = gin.Params{gin.Param{Key: "id", Value: notificationID}}
	c2.Set(middleware.CtxUserIDKey, user.ID)
	handler.MarkRead(c2)

	require.Equal(t, http.StatusOK, readRecorder.Code)

	var readPayload response.Response
	require.NoError(t, json.Unmarshal(readRecorder.Body.Bytes(), &readPayload))
	require.True(t, readPayload.Success)

	readData, err := json.Marshal(readPayload.Data)
	require.NoError(t, err)

	var dto services.NotificationDTO
	require.NoError(t, json.Unmarshal(readData, &dto))
	require.True(t, dto.IsRead)
}

func testContext() context.Context {
	return context.Background()
}
