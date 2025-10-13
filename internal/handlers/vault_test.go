package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/handlers/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestVaultHandlerIdentityLifecycle(t *testing.T) {
	env := testutil.NewEnv(t)

	user := env.CreateRootUser("Password123!")
	login := env.Login(user.Username, "Password123!")

	createPayload := map[string]any{
		"name":        "Production SSH",
		"scope":       "global",
		"payload":     map[string]any{"username": "alice", "private_key": "---"},
		"metadata":    map[string]any{"purpose": "prod"},
		"description": "Root SSH key",
	}

	res := env.Request(http.MethodPost, "/api/vault/identities", createPayload, login.AccessToken)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())

	createdResp := testutil.DecodeResponse(t, res)
	var created services.IdentityDTO
	testutil.DecodeInto(t, createdResp.Data, &created)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "Production SSH", created.Name)

	listRes := env.Request(http.MethodGet, "/api/vault/identities", nil, login.AccessToken)
	require.Equal(t, http.StatusOK, listRes.Code)
	listResp := testutil.DecodeResponse(t, listRes)
	require.True(t, listResp.Success)

	var identities []services.IdentityDTO
	testutil.DecodeInto(t, listResp.Data, &identities)
	require.Len(t, identities, 1)
	require.Equal(t, created.ID, identities[0].ID)

	getRes := env.Request(http.MethodGet, "/api/vault/identities/"+created.ID+"?include=payload", nil, login.AccessToken)
	require.Equal(t, http.StatusOK, getRes.Code, getRes.Body.String())
	getResp := testutil.DecodeResponse(t, getRes)

	var fetched services.IdentityDTO
	testutil.DecodeInto(t, getResp.Data, &fetched)
	require.Equal(t, "Production SSH", fetched.Name)
	require.NotNil(t, fetched.Payload)
	require.Equal(t, "alice", fetched.Payload["username"])

	updatePayload := map[string]any{
		"description": "Updated description",
	}
	updateRes := env.Request(http.MethodPatch, "/api/vault/identities/"+created.ID, updatePayload, login.AccessToken)
	require.Equal(t, http.StatusOK, updateRes.Code, updateRes.Body.String())

	delRes := env.Request(http.MethodDelete, "/api/vault/identities/"+created.ID, nil, login.AccessToken)
	require.Equal(t, http.StatusNoContent, delRes.Code)

	missing := env.Request(http.MethodGet, "/api/vault/identities/"+created.ID, nil, login.AccessToken)
	require.Equal(t, http.StatusNotFound, missing.Code)
}

func TestVaultHandlerTemplates(t *testing.T) {
	env := testutil.NewEnv(t)

	db := env.DB
	fields := []byte(`[{"key":"username","type":"string"}]`)
	protocols := []byte(`["ssh"]`)

	tpl := models.CredentialTemplate{
		DriverID:            "ssh",
		Version:             "1.0.0",
		DisplayName:         "SSH",
		Fields:              fields,
		CompatibleProtocols: protocols,
		Hash:                "hash",
	}
	require.NoError(t, db.Create(&tpl).Error)

	user := env.CreateRootUser("Password123!")
	login := env.Login(user.Username, "Password123!")

	res := env.Request(http.MethodGet, "/api/vault/templates", nil, login.AccessToken)
	require.Equal(t, http.StatusOK, res.Code)
	resp := testutil.DecodeResponse(t, res)
	require.True(t, resp.Success)

	var templates []services.TemplateDTO
	testutil.DecodeInto(t, resp.Data, &templates)
	require.Len(t, templates, 1)
	require.Equal(t, "ssh", templates[0].DriverID)
}
