package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	driverssh "github.com/charlesng35/shellcn/internal/drivers/ssh"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/internal/vault"
	"github.com/charlesng35/shellcn/pkg/response"
)

type launchTestEnv struct {
	db            *gorm.DB
	handler       *ActiveSessionLaunchHandler
	connectionSvc *services.ConnectionService
	active        *services.ActiveSessionService
	lifecycle     *services.SessionLifecycleService
	vaultSvc      *services.VaultService
	checker       *permissions.Checker
	driverReg     *drivers.Registry
	cfg           *app.Config
	jwt           *iauth.JWTService
	crypto        *vault.Crypto
}

func setupLaunchTestEnv(t *testing.T) launchTestEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	require.NoError(t, permissions.Sync(context.Background(), db))

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	vaultSvc, err := services.NewVaultService(db, nil, checker, crypto)
	require.NoError(t, err)

	driverReg := drivers.NewRegistry()
	driverReg.MustRegister(driverssh.NewSSHDriver())

	templateSvc, err := services.NewConnectionTemplateService(db, driverReg)
	require.NoError(t, err)

	connectionSvc, err := services.NewConnectionService(db, checker,
		services.WithConnectionVault(vaultSvc),
		services.WithConnectionTemplates(templateSvc),
	)
	require.NoError(t, err)

	activeSvc := services.NewActiveSessionService(nil)
	lifecycleSvc, err := services.NewSessionLifecycleService(db, activeSvc)
	require.NoError(t, err)

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "launch-secret"})
	require.NoError(t, err)

	cfg := &app.Config{}
	cfg.Protocols.SSH.EnableSFTPDefault = true
	cfg.Features.Recording.Mode = services.RecordingModeOptional
	cfg.Features.Recording.Storage = "filesystem"
	cfg.Features.Recording.RetentionDays = 30
	cfg.Features.Recording.RequireConsent = true
	cfg.Features.Sessions.ConcurrentLimitDefault = 2

	handler := NewActiveSessionLaunchHandler(
		cfg,
		connectionSvc,
		templateSvc,
		vaultSvc,
		lifecycleSvc,
		activeSvc,
		nil,
		driverReg,
		checker,
		jwtSvc,
	)

	return launchTestEnv{
		db:            db,
		handler:       handler,
		connectionSvc: connectionSvc,
		active:        activeSvc,
		lifecycle:     lifecycleSvc,
		vaultSvc:      vaultSvc,
		checker:       checker,
		driverReg:     driverReg,
		cfg:           cfg,
		jwt:           jwtSvc,
		crypto:        crypto,
	}
}

func createRootUser(t *testing.T, db *gorm.DB) models.User {
	t.Helper()
	user := models.User{
		BaseModel: models.BaseModel{ID: "user-" + uuid.NewString()},
		Username:  "root",
		Email:     "root@example.com",
		Password:  "secret",
		IsRoot:    true,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createIdentity(t *testing.T, env launchTestEnv, ownerID string) *models.Identity {
	t.Helper()
	payload := map[string]any{
		"username": "ubuntu",
		"password": "s3cret",
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	cipher, err := env.crypto.Encrypt(raw)
	require.NoError(t, err)

	identity := &models.Identity{
		BaseModel:        models.BaseModel{ID: "identity-" + uuid.NewString()},
		Name:             "SSH Root",
		Scope:            models.IdentityScopeGlobal,
		OwnerUserID:      ownerID,
		Version:          1,
		EncryptedPayload: cipher,
	}
	require.NoError(t, env.db.Create(identity).Error)
	return identity
}

func createConnection(t *testing.T, env launchTestEnv, owner models.User, identity *models.Identity) services.ConnectionDTO {
	t.Helper()

	ctx := context.Background()
	dto, err := env.connectionSvc.Create(ctx, owner.ID, services.CreateConnectionInput{
		Name:       "Primary SSH",
		ProtocolID: driverssh.DriverIDSSH,
		Fields: map[string]any{
			"host":              "host.internal",
			"port":              2222,
			"recording_enabled": true,
		},
		IdentityID: &identity.ID,
	})
	require.NoError(t, err)
	return *dto
}

type launchPayload struct {
	Session struct {
		ID           string         `json:"id"`
		ConnectionID string         `json:"connection_id"`
		DescriptorID string         `json:"descriptor_id"`
		Metadata     map[string]any `json:"metadata"`
	} `json:"session"`
	Tunnel struct {
		URL      string            `json:"url"`
		Token    string            `json:"token"`
		Protocol string            `json:"protocol"`
		Params   map[string]string `json:"params"`
	} `json:"tunnel"`
	Descriptor struct {
		ID          string `json:"id"`
		ProtocolID  string `json:"protocol_id"`
		DisplayName string `json:"display_name"`
	} `json:"descriptor"`
	TemplateMismatch bool `json:"template_mismatch"`
}

func performLaunch(t *testing.T, env launchTestEnv, user models.User, conn services.ConnectionDTO, body map[string]any) (*httptest.ResponseRecorder, launchPayload) {
	t.Helper()

	payloadBytes, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, err := http.NewRequest(http.MethodPost, "/api/active-sessions", bytes.NewReader(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	claims := &iauth.Claims{
		UserID: user.ID,
		Metadata: map[string]any{
			"username": user.Username,
			"email":    user.Email,
			"is_root":  true,
		},
	}
	c.Set(middleware.CtxUserIDKey, user.ID)
	c.Set(middleware.CtxClaimsKey, claims)

	env.handler.Launch(c)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	var result launchPayload
	require.NoError(t, decodeEnvelopeData(envelope.Data, &result))
	return w, result
}

func decodeEnvelopeData(data interface{}, target interface{}) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func TestActiveSessionLaunch_Success(t *testing.T) {
	env := setupLaunchTestEnv(t)

	user := createRootUser(t, env.db)
	identity := createIdentity(t, env, user.ID)
	conn := createConnection(t, env, user, identity)

	w, payload := performLaunch(t, env, user, conn, map[string]any{
		"connection_id": conn.ID,
	})

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotEmpty(t, payload.Session.ID)
	require.Equal(t, conn.ID, payload.Session.ConnectionID)
	require.Equal(t, workspaceDescriptorID(conn.ProtocolID), payload.Session.DescriptorID)
	require.False(t, payload.TemplateMismatch)
	require.Equal(t, "ssh", payload.Tunnel.Protocol)
	require.NotEmpty(t, payload.Tunnel.Token)
	require.Equal(t, payload.Session.ID, payload.Tunnel.Params["session_id"])

	capabilities, ok := payload.Session.Metadata["capabilities"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, capabilities, "features")

	var session models.ConnectionSession
	require.NoError(t, env.db.First(&session, "id = ?", payload.Session.ID).Error)
	require.Equal(t, conn.ID, session.ConnectionID)

	active := env.active.ListActive(services.ListActiveOptions{UserID: user.ID})
	require.Len(t, active, 1)
	require.Equal(t, payload.Session.ID, active[0].ID)
}

func TestActiveSessionLaunch_TemplateMismatchFlag(t *testing.T) {
	env := setupLaunchTestEnv(t)

	user := createRootUser(t, env.db)
	identity := createIdentity(t, env, user.ID)
	conn := createConnection(t, env, user, identity)

	// Update stored template version to simulate mismatch.
	var model models.Connection
	require.NoError(t, env.db.First(&model, "id = ?", conn.ID).Error)
	meta := map[string]any{}
	require.NoError(t, json.Unmarshal(model.Metadata, &meta))
	if template, ok := meta["connection_template"].(map[string]any); ok {
		template["version"] = "2024-01-01"
		meta["connection_template"] = template
		payload, err := json.Marshal(meta)
		require.NoError(t, err)
		require.NoError(t, env.db.Model(&models.Connection{}).
			Where("id = ?", conn.ID).
			Update("metadata", datatypes.JSON(payload)).Error)
	}

	w, payload := performLaunch(t, env, user, conn, map[string]any{
		"connection_id": conn.ID,
	})

	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, payload.TemplateMismatch)
	templateMeta, ok := payload.Session.Metadata["template"].(map[string]any)
	require.True(t, ok)
	mismatch, ok := templateMeta["version_mismatch"].(bool)
	require.True(t, ok)
	require.True(t, mismatch)
}
