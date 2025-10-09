package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestConnectionHandlerList(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-handler"},
		Username:  "handler",
		Email:     "handler@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Handler SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		Targets: []models.ConnectionTarget{
			{
				Host:   "10.10.0.1",
				Port:   22,
				Labels: datatypes.JSON([]byte(`{"env":"prod"}`)),
			},
		},
	}).Error)

	svc, err := services.NewConnectionService(db, &fakePermissionChecker{
		grants: map[string]bool{
			"connection.view": true,
		},
	})
	require.NoError(t, err)

	handler := NewConnectionHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(middleware.CtxUserIDKey, user.ID)
	handler.List(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, true, payload["success"])
}

type fakePermissionChecker struct {
	grants map[string]bool
	err    error
}

func (f *fakePermissionChecker) Check(_ context.Context, _ string, permissionID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.grants[permissionID], nil
}
