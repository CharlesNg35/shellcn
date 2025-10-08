package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/handlers/testutil"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestSetupHandler_StatusAndInitialize(t *testing.T) {
	env := testutil.NewEnv(t)

	statusResp := env.Request(http.MethodGet, "/api/setup/status", nil, "")
	require.Equal(t, http.StatusOK, statusResp.Code)

	statusPayload := testutil.DecodeResponse(t, statusResp)
	require.True(t, statusPayload.Success)
	var status map[string]bool
	testutil.DecodeInto(t, statusPayload.Data, &status)
	require.False(t, status["initialized"])

	body := map[string]any{
		"username":   "initial-root",
		"email":      "initial@example.com",
		"password":   "SuperSecret123!",
		"first_name": "Initial",
		"last_name":  "Root",
	}
	initResp := env.Request(http.MethodPost, "/api/setup/initialize", body, "")
	require.Equal(t, http.StatusCreated, initResp.Code)

	initPayload := testutil.DecodeResponse(t, initResp)
	require.True(t, initPayload.Success)
	var initData map[string]string
	testutil.DecodeInto(t, initPayload.Data, &initData)
	require.NotEmpty(t, initData["root_user_id"])

	statusResp = env.Request(http.MethodGet, "/api/setup/status", nil, "")
	statusPayload = testutil.DecodeResponse(t, statusResp)
	require.True(t, statusPayload.Success)
	testutil.DecodeInto(t, statusPayload.Data, &status)
	require.True(t, status["initialized"])

	// Re-initialisation should fail with conflict.
	again := env.Request(http.MethodPost, "/api/setup/initialize", body, "")
	require.Equal(t, http.StatusConflict, again.Code)
	againPayload := testutil.DecodeResponse(t, again)
	require.False(t, againPayload.Success)
	require.NotNil(t, againPayload.Error)
	require.Equal(t, "ALREADY_INITIALIZED", againPayload.Error.Code)
}

func TestUserHandler_ListGetCreate(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("StrongPassw0rd!")

	// unauthenticated request should fail
	unauth := env.Request(http.MethodGet, "/api/users", nil, "")
	require.Equal(t, http.StatusUnauthorized, unauth.Code)

	login := env.Login(root.Username, "StrongPassw0rd!")
	token := login.Tokens.AccessToken

	list := env.Request(http.MethodGet, "/api/users", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	listPayload := testutil.DecodeResponse(t, list)
	require.True(t, listPayload.Success)
	require.NotNil(t, listPayload.Meta)
	require.GreaterOrEqual(t, listPayload.Meta.Total, 1)

	createPayload := map[string]any{
		"username": "alice-" + uuid.NewString(),
		"email":    "alice-" + uuid.NewString() + "@example.com",
		"password": "Password123!",
		"is_root":  false,
	}
	created := env.Request(http.MethodPost, "/api/users", createPayload, token)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	createResp := testutil.DecodeResponse(t, created)
	require.True(t, createResp.Success)

	var createdUser map[string]any
	testutil.DecodeInto(t, createResp.Data, &createdUser)
	userID, ok := createdUser["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, userID)

	get := env.Request(http.MethodGet, "/api/users/"+userID, nil, token)
	require.Equal(t, http.StatusOK, get.Code)
	getResp := testutil.DecodeResponse(t, get)
	require.True(t, getResp.Success)
	var fetched map[string]any
	testutil.DecodeInto(t, getResp.Data, &fetched)
	require.Equal(t, createdUser["username"], fetched["username"])
	require.Equal(t, createdUser["email"], fetched["email"])
}

func TestOrganizationHandler_CRUD(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("OrgPassw0rd!")
	login := env.Login(root.Username, "OrgPassw0rd!")
	token := login.Tokens.AccessToken

	createPayload := map[string]any{
		"name":        "Test Org",
		"description": "organization used in tests",
	}
	created := env.Request(http.MethodPost, "/api/orgs", createPayload, token)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	createResp := testutil.DecodeResponse(t, created)
	require.True(t, createResp.Success)

	var org map[string]any
	testutil.DecodeInto(t, createResp.Data, &org)
	orgID := org["id"].(string)

	list := env.Request(http.MethodGet, "/api/orgs", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	listResp := testutil.DecodeResponse(t, list)
	require.True(t, listResp.Success)
	var orgs []map[string]any
	testutil.DecodeInto(t, listResp.Data, &orgs)
	require.NotEmpty(t, orgs)

	get := env.Request(http.MethodGet, "/api/orgs/"+orgID, nil, token)
	require.Equal(t, http.StatusOK, get.Code)

	updatePayload := map[string]any{"description": "updated description"}
	updated := env.Request(http.MethodPatch, "/api/orgs/"+orgID, updatePayload, token)
	require.Equal(t, http.StatusOK, updated.Code)
	updateResp := testutil.DecodeResponse(t, updated)
	var updatedOrg map[string]any
	testutil.DecodeInto(t, updateResp.Data, &updatedOrg)
	require.Equal(t, "updated description", updatedOrg["description"])

	deleteResp := env.Request(http.MethodDelete, "/api/orgs/"+orgID, nil, token)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	// ensure deleted
	getAfterDelete := env.Request(http.MethodGet, "/api/orgs/"+orgID, nil, token)
	require.Equal(t, http.StatusNotFound, getAfterDelete.Code)
	errPayload := testutil.DecodeResponse(t, getAfterDelete)
	require.False(t, errPayload.Success)
	require.NotNil(t, errPayload.Error)
	require.Equal(t, "NOT_FOUND", errPayload.Error.Code)
}

func TestTeamHandler_Flow(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("TeamPassw0rd!")
	login := env.Login(root.Username, "TeamPassw0rd!")
	token := login.Tokens.AccessToken

	orgPayload := map[string]any{
		"name":        "Team Org",
		"description": "org for team tests",
	}
	orgResp := env.Request(http.MethodPost, "/api/orgs", orgPayload, token)
	require.Equal(t, http.StatusCreated, orgResp.Code)
	var org map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, orgResp).Data, &org)
	orgID := org["id"].(string)

	teamPayload := map[string]any{
		"organization_id": orgID,
		"name":            "Platform",
		"description":     "platform team",
	}
	teamResp := env.Request(http.MethodPost, "/api/teams", teamPayload, token)
	require.Equal(t, http.StatusCreated, teamResp.Code, teamResp.Body.String())
	var team map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, teamResp).Data, &team)
	teamID := team["id"].(string)

	getTeam := env.Request(http.MethodGet, "/api/teams/"+teamID, nil, token)
	require.Equal(t, http.StatusOK, getTeam.Code)

	updatePayload := map[string]any{"description": "new description"}
	updateResp := env.Request(http.MethodPatch, "/api/teams/"+teamID, updatePayload, token)
	require.Equal(t, http.StatusOK, updateResp.Code)

	auditSvc, err := services.NewAuditService(env.DB)
	require.NoError(t, err)
	userSvc, err := services.NewUserService(env.DB, auditSvc)
	require.NoError(t, err)
	memberUser, err := userSvc.Create(context.Background(), services.CreateUserInput{
		Username: "member-" + uuid.NewString(),
		Email:    "member-" + uuid.NewString() + "@example.com",
		Password: "MemberPass123!",
		IsRoot:   false,
	})
	require.NoError(t, err)
	memberID := memberUser.ID

	addMemberPayload := map[string]any{"user_id": memberID}
	addResp := env.Request(http.MethodPost, "/api/teams/"+teamID+"/members", addMemberPayload, token)
	require.Equal(t, http.StatusOK, addResp.Code)

	listMembers := env.Request(http.MethodGet, "/api/teams/"+teamID+"/members", nil, token)
	require.Equal(t, http.StatusOK, listMembers.Code)
	var members []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, listMembers).Data, &members)
	require.Len(t, members, 1)
	require.Equal(t, memberID, members[0]["id"])

	orgTeams := env.Request(http.MethodGet, "/api/organizations/"+orgID+"/teams", nil, token)
	require.Equal(t, http.StatusOK, orgTeams.Code)
	var teams []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, orgTeams).Data, &teams)
	require.NotEmpty(t, teams)

	removeResp := env.Request(http.MethodDelete, "/api/teams/"+teamID+"/members/"+memberID, nil, token)
	require.Equal(t, http.StatusOK, removeResp.Code)
}

func TestSessionHandler_ListAndRevoke(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("SessionPassw0rd!")
	login := env.Login(root.Username, "SessionPassw0rd!")
	token := login.Tokens.AccessToken

	list := env.Request(http.MethodGet, "/api/sessions/me", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	var sessions []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, list).Data, &sessions)
	require.NotEmpty(t, sessions)
	sessionID := sessions[0]["id"].(string)

	revoke := env.Request(http.MethodPost, "/api/sessions/revoke/"+sessionID, nil, token)
	require.Equal(t, http.StatusOK, revoke.Code)

	revokeAll := env.Request(http.MethodPost, "/api/sessions/revoke_all", nil, token)
	require.Equal(t, http.StatusOK, revokeAll.Code)
}

func TestPermissionHandler_RoleLifecycle(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("PermPassw0rd!")
	login := env.Login(root.Username, "PermPassw0rd!")
	token := login.Tokens.AccessToken

	registry := env.Request(http.MethodGet, "/api/permissions/registry", nil, token)
	require.Equal(t, http.StatusOK, registry.Code)
	var perms map[string]map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, registry).Data, &perms)
	require.NotEmpty(t, perms)

	roles := env.Request(http.MethodGet, "/api/permissions/roles", nil, token)
	require.Equal(t, http.StatusOK, roles.Code)
	var existing []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, roles).Data, &existing)
	require.NotEmpty(t, existing)

	rolePayload := map[string]any{
		"name":        "Support",
		"description": "support engineers",
	}
	createRole := env.Request(http.MethodPost, "/api/permissions/roles", rolePayload, token)
	require.Equal(t, http.StatusCreated, createRole.Code, createRole.Body.String())
	var role map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, createRole).Data, &role)
	roleID := role["id"].(string)

	updatePayload := map[string]any{
		"name":        "Support Updated",
		"description": "updated description",
	}
	updateRole := env.Request(http.MethodPatch, "/api/permissions/roles/"+roleID, updatePayload, token)
	require.Equal(t, http.StatusOK, updateRole.Code)

	assignPayload := map[string]any{"permissions": []string{"user.view"}}
	assign := env.Request(http.MethodPost, "/api/permissions/roles/"+roleID+"/permissions", assignPayload, token)
	require.Equal(t, http.StatusOK, assign.Code)

	deleteRole := env.Request(http.MethodDelete, "/api/permissions/roles/"+roleID, nil, token)
	require.Equal(t, http.StatusOK, deleteRole.Code)
}

func TestAuditHandler_ListAndExport(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("AuditPassw0rd!")
	login := env.Login(root.Username, "AuditPassw0rd!")
	token := login.Tokens.AccessToken

	// Trigger some audit events by creating a user.
	createPayload := map[string]any{
		"username": "audited-" + uuid.NewString(),
		"email":    "audited-" + uuid.NewString() + "@example.com",
		"password": "AuditUser123!",
	}
	createResp := env.Request(http.MethodPost, "/api/users", createPayload, token)
	require.Equal(t, http.StatusCreated, createResp.Code)

	list := env.Request(http.MethodGet, "/api/audit?page=1&per_page=10", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	listPayload := testutil.DecodeResponse(t, list)
	require.True(t, listPayload.Success)
	require.NotNil(t, listPayload.Meta)

	var entries []map[string]any
	testutil.DecodeInto(t, listPayload.Data, &entries)
	require.NotEmpty(t, entries)

	export := env.Request(http.MethodGet, "/api/audit/export", nil, token)
	require.Equal(t, http.StatusOK, export.Code)
	exportPayload := testutil.DecodeResponse(t, export)
	require.True(t, exportPayload.Success)
	var exportEntries []map[string]any
	testutil.DecodeInto(t, exportPayload.Data, &exportEntries)
	require.NotEmpty(t, exportEntries)
}

func TestAuthProviderHandler_Flow(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("ProvidersPassw0rd!")
	login := env.Login(root.Username, "ProvidersPassw0rd!")
	token := login.Tokens.AccessToken

	list := env.Request(http.MethodGet, "/api/auth/providers", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	var providers []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, list).Data, &providers)
	require.NotEmpty(t, providers)

	enabled := env.Request(http.MethodGet, "/api/auth/providers/enabled", nil, token)
	require.Equal(t, http.StatusOK, enabled.Code)

	localPayload := map[string]any{
		"allow_registration":         true,
		"require_email_verification": false,
	}
	localResp := env.Request(http.MethodPost, "/api/auth/providers/local/settings", localPayload, token)
	require.Equal(t, http.StatusOK, localResp.Code, localResp.Body.String())

	oidcPayload := map[string]any{
		"enabled": true,
		"config": map[string]any{
			"issuer":        "https://accounts.example.com",
			"client_id":     "oidc-client",
			"client_secret": "super-secret",
			"redirect_url":  "https://app.example.com/callback",
			"scopes":        []string{"openid", "profile", "email"},
		},
	}
	oidcConfig := env.Request(http.MethodPost, "/api/auth/providers/oidc/configure", oidcPayload, token)
	require.Equal(t, http.StatusOK, oidcConfig.Code, oidcConfig.Body.String())

	enableOIDC := env.Request(http.MethodPost, "/api/auth/providers/oidc/enable", map[string]any{"enabled": true}, token)
	require.Equal(t, http.StatusOK, enableOIDC.Code)

	testOIDC := env.Request(http.MethodPost, "/api/auth/providers/oidc/test", nil, token)
	require.Equal(t, http.StatusOK, testOIDC.Code, testOIDC.Body.String())

	ldapPayload := map[string]any{
		"enabled": true,
		"config": map[string]any{
			"host":          "ldap.example.com",
			"port":          389,
			"base_dn":       "dc=example,dc=com",
			"bind_dn":       "cn=admin,dc=example,dc=com",
			"bind_password": "ldap-secret",
			"user_filter":   "(uid={username})",
			"use_tls":       false,
			"skip_verify":   true,
			"attribute_mapping": map[string]string{
				"email": "mail",
			},
		},
	}
	ldapConfig := env.Request(http.MethodPost, "/api/auth/providers/ldap/configure", ldapPayload, token)
	require.Equal(t, http.StatusOK, ldapConfig.Code, ldapConfig.Body.String())

	enableLDAP := env.Request(http.MethodPost, "/api/auth/providers/ldap/enable", map[string]any{"enabled": true}, token)
	require.Equal(t, http.StatusOK, enableLDAP.Code)

	testLDAP := env.Request(http.MethodPost, "/api/auth/providers/ldap/test", nil, token)
	require.Equal(t, http.StatusOK, testLDAP.Code, testLDAP.Body.String())
}
