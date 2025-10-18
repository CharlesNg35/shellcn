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
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
)

type handlerPermissionStub struct {
	grants map[string]bool
}

func (h *handlerPermissionStub) Check(ctx context.Context, userID, permissionID string) (bool, error) {
	return h.grants[permissionID], nil
}

func (h *handlerPermissionStub) CheckResource(ctx context.Context, userID, resourceType, resourceID, permissionID string) (bool, error) {
	return h.grants[permissionID], nil
}

func TestProtocolHandlerListAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureHandlerProtocolPermissions(t)
	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	svc, err := services.NewProtocolService(db, nil)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc, nil)

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
	ensureHandlerProtocolPermissions(t)
	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	checker := &handlerPermissionStub{grants: map[string]bool{
		"connection.view":      true,
		"protocol:ssh.connect": true,
	}}
	svc, err := services.NewProtocolService(db, checker)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/protocols/available", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set(middleware.CtxUserIDKey, "user-123")

	handler.ListForUser(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestProtocolHandlerListPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureHandlerProtocolPermissions(t)
	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	svc, err := services.NewProtocolService(db, nil)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/protocols/ssh/permissions", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "ssh"}}

	handler.ListPermissions(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestProtocolHandlerGetConnectionTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureHandlerProtocolPermissions(t)

	insertProtocolRecord(t, db, models.ConnectionProtocol{ProtocolID: "ssh", Name: "SSH", Module: "ssh", DriverEnabled: true, ConfigEnabled: true}, drivers.Capabilities{Terminal: true})

	sections := []map[string]any{
		{
			"id":     "endpoint",
			"label":  "Endpoint",
			"fields": []map[string]any{{"key": "host", "label": "Host", "type": "string", "required": true}},
		},
	}
	sectionsJSON, err := json.Marshal(sections)
	require.NoError(t, err)
	metadataJSON, err := json.Marshal(map[string]any{"requires_identity": true})
	require.NoError(t, err)

	require.NoError(t, db.Create(&models.ConnectionTemplate{
		DriverID:    "ssh",
		Version:     "1.0.0",
		DisplayName: "SSH Connection",
		Sections:    datatypes.JSON(sectionsJSON),
		Metadata:    datatypes.JSON(metadataJSON),
	}).Error)

	templateSvc, err := services.NewConnectionTemplateService(db, drivers.NewRegistry())
	require.NoError(t, err)
	svc, err := services.NewProtocolService(db, nil)
	require.NoError(t, err)
	handler := NewProtocolHandler(svc, templateSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/protocols/ssh/connection-template", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "ssh"}}

	handler.GetConnectionTemplate(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
}

func insertProtocolRecord(t *testing.T, db *gorm.DB, record models.ConnectionProtocol, caps drivers.Capabilities) {
	t.Helper()
	featuresJSON, err := json.Marshal(deriveHandlerFeatures(caps))
	require.NoError(t, err)
	capsJSON, err := json.Marshal(caps)
	require.NoError(t, err)

	record.Features = datatypes.JSON(featuresJSON)
	record.Capabilities = datatypes.JSON(capsJSON)
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

func ensureHandlerProtocolPermissions(t *testing.T) {
	t.Helper()

	if _, ok := permissions.Get("protocol:ssh.connect"); !ok {
		require.NoError(t, permissions.RegisterProtocolPermission("ssh", "connect", &permissions.Permission{
			DisplayName:  "SSH Connect",
			Description:  "Initiate SSH sessions",
			DefaultScope: "resource",
			DependsOn:    []string{"connection.launch"},
		}))
	}

	if _, ok := permissions.Get("protocol:ssh.port_forward"); !ok {
		require.NoError(t, permissions.RegisterProtocolPermission("ssh", "port_forward", &permissions.Permission{
			DisplayName:  "SSH Port Forward",
			Description:  "Forward ports through SSH tunnels",
			DefaultScope: "resource",
			DependsOn:    []string{"protocol:ssh.connect"},
		}))
	}
}
