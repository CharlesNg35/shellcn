package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
)

type handlerPermissionStub struct {
	grants map[string]bool
}

func (h *handlerPermissionStub) Check(ctx context.Context, userID, permissionID string) (bool, error) {
	return h.grants[permissionID], nil
}

func TestProtocolHandlerListAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	svc, err := services.NewProtocolService(db, nil)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/protocols", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	handler.ListAll(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestProtocolHandlerListForUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	checker := &handlerPermissionStub{grants: map[string]bool{
		"connection.view": true,
		"ssh.connect":     true,
	}}
	svc, err := services.NewProtocolService(db, checker)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/protocols/available", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set(middleware.CtxUserIDKey, "user-123")

	handler.ListForUser(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
}

func insertProtocolRecord(t *testing.T, db *gorm.DB, record models.ConnectionProtocol, caps drivers.Capabilities) {
	t.Helper()
	featuresJSON, err := json.Marshal(deriveHandlerFeatures(caps))
	require.NoError(t, err)
	capsJSON, err := json.Marshal(caps)
	require.NoError(t, err)

	record.Features = string(featuresJSON)
	record.Capabilities = string(capsJSON)
	record.DriverID = record.ProtocolID

	require.NoError(t, db.Create(&record).Error)
}

func deriveHandlerFeatures(caps drivers.Capabilities) []string {
	features := make([]string, 0, 8)
	if caps.Terminal {
		features = append(features, "terminal")
	}
	if caps.Desktop {
		features = append(features, "desktop")
	}
	return features
}
