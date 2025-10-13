package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/handlers/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestSetupHandler_StatusAndInitialize(t *testing.T) {
	env := testutil.NewEnv(t)

	statusResp := env.Request(http.MethodGet, "/api/setup/status", nil, "")
	require.Equal(t, http.StatusOK, statusResp.Code)

	statusPayload := testutil.DecodeResponse(t, statusResp)
	require.True(t, statusPayload.Success)
	var status map[string]any
	testutil.DecodeInto(t, statusPayload.Data, &status)
	require.Equal(t, "pending", status["status"])

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
	require.Equal(t, "complete", status["status"])

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
	token := login.AccessToken

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

func TestUserHandler_CreateValidation(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("ValidPassw0rd!")
	login := env.Login(root.Username, "ValidPassw0rd!")
	token := login.AccessToken

	invalidPayload := map[string]any{
		"username": " ",
		"email":    "invalid-email",
		"password": "short",
	}

	resp := env.Request(http.MethodPost, "/api/users", invalidPayload, token)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	decoded := testutil.DecodeResponse(t, resp)
	require.False(t, decoded.Success)
	require.NotNil(t, decoded.Error)
	require.Equal(t, "BAD_REQUEST", decoded.Error.Code)
}

func TestUserHandler_UpdateActivateDelete(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("RootManage123!")
	login := env.Login(root.Username, "RootManage123!")
	token := login.AccessToken

	createPayload := map[string]any{
		"username":   "manage-" + uuid.NewString(),
		"email":      "manage-" + uuid.NewString() + "@example.com",
		"password":   "Password123!",
		"first_name": "First",
		"last_name":  "User",
	}
	createResp := env.Request(http.MethodPost, "/api/users", createPayload, token)
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	createDecoded := testutil.DecodeResponse(t, createResp)
	require.True(t, createDecoded.Success)
	var created map[string]any
	testutil.DecodeInto(t, createDecoded.Data, &created)
	userID := created["id"].(string)

	updatePayload := map[string]any{
		"first_name": "Updated",
		"last_name":  "Person",
		"avatar":     "https://example.com/avatar.png",
	}
	updateResp := env.Request(http.MethodPatch, fmt.Sprintf("/api/users/%s", userID), updatePayload, token)
	require.Equal(t, http.StatusOK, updateResp.Code, updateResp.Body.String())
	updateDecoded := testutil.DecodeResponse(t, updateResp)
	require.True(t, updateDecoded.Success)
	var updated map[string]any
	testutil.DecodeInto(t, updateDecoded.Data, &updated)
	require.Equal(t, "Updated", updated["first_name"])
	require.Equal(t, "Person", updated["last_name"])

	deactivateResp := env.Request(http.MethodPost, fmt.Sprintf("/api/users/%s/deactivate", userID), nil, token)
	require.Equal(t, http.StatusOK, deactivateResp.Code, deactivateResp.Body.String())
	deactivateDecoded := testutil.DecodeResponse(t, deactivateResp)
	require.True(t, deactivateDecoded.Success)
	var deactivated map[string]any
	testutil.DecodeInto(t, deactivateDecoded.Data, &deactivated)
	require.False(t, deactivated["is_active"].(bool))

	activateResp := env.Request(http.MethodPost, fmt.Sprintf("/api/users/%s/activate", userID), nil, token)
	require.Equal(t, http.StatusOK, activateResp.Code, activateResp.Body.String())
	activateDecoded := testutil.DecodeResponse(t, activateResp)
	require.True(t, activateDecoded.Success)
	var activated map[string]any
	testutil.DecodeInto(t, activateDecoded.Data, &activated)
	require.True(t, activated["is_active"].(bool))

	passwordPayload := map[string]any{"password": "NewPassword567!"}
	passwordResp := env.Request(http.MethodPost, fmt.Sprintf("/api/users/%s/password", userID), passwordPayload, token)
	require.Equal(t, http.StatusOK, passwordResp.Code, passwordResp.Body.String())

	deleteResp := env.Request(http.MethodDelete, fmt.Sprintf("/api/users/%s", userID), nil, token)
	require.Equal(t, http.StatusOK, deleteResp.Code, deleteResp.Body.String())

	getDeleted := env.Request(http.MethodGet, fmt.Sprintf("/api/users/%s", userID), nil, token)
	require.Equal(t, http.StatusNotFound, getDeleted.Code)

	rootDeactivate := env.Request(http.MethodPost, fmt.Sprintf("/api/users/%s/deactivate", root.ID), nil, token)
	require.Equal(t, http.StatusBadRequest, rootDeactivate.Code)
	rootDeactivateDecoded := testutil.DecodeResponse(t, rootDeactivate)
	require.False(t, rootDeactivateDecoded.Success)
	require.Equal(t, "BAD_REQUEST", rootDeactivateDecoded.Error.Code)
}

func TestUserHandler_BulkOperations(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("BulkRoot123!")
	login := env.Login(root.Username, "BulkRoot123!")
	token := login.AccessToken

	var userIDs []string
	for i := 0; i < 3; i++ {
		payload := map[string]any{
			"username": fmt.Sprintf("bulk-%d-%s", i, uuid.NewString()),
			"email":    fmt.Sprintf("bulk-%d-%s@example.com", i, uuid.NewString()),
			"password": "BulkPassword123!",
		}
		createResp := env.Request(http.MethodPost, "/api/users", payload, token)
		require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
		var created map[string]any
		testutil.DecodeInto(t, testutil.DecodeResponse(t, createResp).Data, &created)
		userIDs = append(userIDs, created["id"].(string))
	}

	deactivatePayload := map[string]any{
		"user_ids": userIDs,
	}
	deactivateResp := env.Request(http.MethodPost, "/api/users/bulk/deactivate", deactivatePayload, token)
	require.Equal(t, http.StatusOK, deactivateResp.Code, deactivateResp.Body.String())
	deactivateDecoded := testutil.DecodeResponse(t, deactivateResp)
	require.True(t, deactivateDecoded.Success)

	for _, id := range userIDs {
		getResp := env.Request(http.MethodGet, fmt.Sprintf("/api/users/%s", id), nil, token)
		require.Equal(t, http.StatusOK, getResp.Code)
		var fetched map[string]any
		testutil.DecodeInto(t, testutil.DecodeResponse(t, getResp).Data, &fetched)
		require.False(t, fetched["is_active"].(bool))
	}

	activateResp := env.Request(http.MethodPost, "/api/users/bulk/activate", deactivatePayload, token)
	require.Equal(t, http.StatusOK, activateResp.Code, activateResp.Body.String())

	deleteResp := env.Request(http.MethodDelete, "/api/users/bulk", deactivatePayload, token)
	require.Equal(t, http.StatusOK, deleteResp.Code, deleteResp.Body.String())

	for _, id := range userIDs {
		getResp := env.Request(http.MethodGet, fmt.Sprintf("/api/users/%s", id), nil, token)
		require.Equal(t, http.StatusNotFound, getResp.Code)
	}
}

func TestTeamHandler_Flow(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("TeamPassw0rd!")
	login := env.Login(root.Username, "TeamPassw0rd!")
	token := login.AccessToken

	teamPayload := map[string]any{
		"name":        "Platform",
		"description": "platform team",
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

	removeResp := env.Request(http.MethodDelete, "/api/teams/"+teamID+"/members/"+memberID, nil, token)
	require.Equal(t, http.StatusOK, removeResp.Code)
}

func TestSessionHandler_ListAndRevoke(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("SessionPassw0rd!")
	login := env.Login(root.Username, "SessionPassw0rd!")
	token := login.AccessToken

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
	token := login.AccessToken

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
	token := login.AccessToken

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
	token := login.AccessToken

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

	localDetails := env.Request(http.MethodGet, "/api/auth/providers/local", nil, token)
	require.Equal(t, http.StatusOK, localDetails.Code, localDetails.Body.String())
	var localDetail struct {
		Provider map[string]any `json:"provider"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, localDetails).Data, &localDetail)
	require.Equal(t, "local", localDetail.Provider["type"])
	require.Equal(t, true, localDetail.Provider["allow_registration"])

	oidcPayload := map[string]any{
		"enabled":            true,
		"allow_registration": true,
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

	oidcDetails := env.Request(http.MethodGet, "/api/auth/providers/oidc", nil, token)
	require.Equal(t, http.StatusOK, oidcDetails.Code, oidcDetails.Body.String())
	var oidcDetail struct {
		Provider map[string]any `json:"provider"`
		Config   map[string]any `json:"config"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, oidcDetails).Data, &oidcDetail)
	require.Equal(t, "oidc", oidcDetail.Provider["type"])
	require.Equal(t, "https://accounts.example.com", oidcDetail.Config["issuer"])
	require.Equal(t, "oidc-client", oidcDetail.Config["client_id"])

	ldapPayload := map[string]any{
		"enabled":            true,
		"allow_registration": false,
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

	ldapDetails := env.Request(http.MethodGet, "/api/auth/providers/ldap", nil, token)
	require.Equal(t, http.StatusOK, ldapDetails.Code, ldapDetails.Body.String())
	var ldapDetail struct {
		Provider map[string]any `json:"provider"`
		Config   map[string]any `json:"config"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, ldapDetails).Data, &ldapDetail)
	require.Equal(t, "ldap", ldapDetail.Provider["type"])
	require.Equal(t, "ldap.example.com", ldapDetail.Config["host"])
}

func TestInviteHandler_Flow(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("InviteFlowPassw0rd!")
	login := env.Login(root.Username, "InviteFlowPassw0rd!")
	token := login.AccessToken

	teamPayload := map[string]any{
		"name":        "Onboarding",
		"description": "handles new hires",
	}
	teamResp := env.Request(http.MethodPost, "/api/teams", teamPayload, token)
	require.Equal(t, http.StatusCreated, teamResp.Code, teamResp.Body.String())
	var team map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, teamResp).Data, &team)
	teamID := team["id"].(string)

	createPayload := map[string]any{
		"email":   "invitee@example.com",
		"team_id": teamID,
	}
	createResp := env.Request(http.MethodPost, "/api/invites", createPayload, token)
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())

	var createData struct {
		Invite map[string]any `json:"invite"`
		Token  string         `json:"token"`
		Link   string         `json:"link"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, createResp).Data, &createData)
	require.NotEmpty(t, createData.Token)
	require.Equal(t, "invitee@example.com", createData.Invite["email"])
	require.Equal(t, teamID, createData.Invite["team_id"])
	require.Equal(t, "Onboarding", createData.Invite["team_name"])
	inviteID := createData.Invite["id"].(string)

	linkResp := env.Request(http.MethodPost, "/api/invites/"+inviteID+"/link", nil, token)
	require.Equal(t, http.StatusOK, linkResp.Code, linkResp.Body.String())
	var linkData struct {
		Invite map[string]any `json:"invite"`
		Token  string         `json:"token"`
		Link   string         `json:"link"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, linkResp).Data, &linkData)
	require.NotEmpty(t, linkData.Token)
	require.NotEmpty(t, linkData.Link)
	require.Equal(t, inviteID, linkData.Invite["id"])
	require.NotEqual(t, createData.Token, linkData.Token)

	resendResp := env.Request(http.MethodPost, "/api/invites/"+inviteID+"/resend", nil, token)
	require.Equal(t, http.StatusOK, resendResp.Code, resendResp.Body.String())
	var resendData struct {
		Invite map[string]any `json:"invite"`
		Token  string         `json:"token"`
		Link   string         `json:"link"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, resendResp).Data, &resendData)
	require.NotEmpty(t, resendData.Token)
	require.NotEmpty(t, resendData.Link)
	require.Equal(t, inviteID, resendData.Invite["id"])
	require.NotEqual(t, linkData.Token, resendData.Token)

	listResp := env.Request(http.MethodGet, "/api/invites", nil, token)
	require.Equal(t, http.StatusOK, listResp.Code)
	var listPayload struct {
		Invites []map[string]any `json:"invites"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, listResp).Data, &listPayload)
	require.NotEmpty(t, listPayload.Invites)
	require.Equal(t, teamID, listPayload.Invites[0]["team_id"])

	redeemPayload := map[string]any{
		"token":      resendData.Token,
		"username":   "invited-user",
		"password":   "InviteePassword123!",
		"first_name": "Invited",
		"last_name":  "User",
	}
	redeemResp := env.Request(http.MethodPost, "/api/auth/invite/redeem", redeemPayload, "")
	require.Equal(t, http.StatusCreated, redeemResp.Code, redeemResp.Body.String())

	// New user should be able to authenticate immediately.
	loginResult := env.Login("invited-user", "InviteePassword123!")
	require.NotEmpty(t, loginResult.AccessToken)

	// Listing invites should show status updated.
	listResp = env.Request(http.MethodGet, "/api/invites", nil, token)
	require.Equal(t, http.StatusOK, listResp.Code)
	testutil.DecodeInto(t, testutil.DecodeResponse(t, listResp).Data, &listPayload)
	require.NotEmpty(t, listPayload.Invites)
	require.Equal(t, "accepted", listPayload.Invites[0]["status"])

	memberList := env.Request(http.MethodGet, "/api/teams/"+teamID+"/members", nil, token)
	require.Equal(t, http.StatusOK, memberList.Code, memberList.Body.String())
	var members []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, memberList).Data, &members)
	found := false
	for _, member := range members {
		if member["username"] == "invited-user" {
			found = true
			break
		}
	}
	require.True(t, found, "invited user should be added to the team")
}

func TestInviteHandler_TeamInviteExistingUser(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("ExistingInvitePassw0rd!")
	login := env.Login(root.Username, "ExistingInvitePassw0rd!")
	token := login.AccessToken

	teamPayload := map[string]any{
		"name":        "Customer Success",
		"description": "supports customers",
	}
	teamResp := env.Request(http.MethodPost, "/api/teams", teamPayload, token)
	require.Equal(t, http.StatusCreated, teamResp.Code, teamResp.Body.String())

	var team map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, teamResp).Data, &team)
	teamID := team["id"].(string)

	auditSvc, err := services.NewAuditService(env.DB)
	require.NoError(t, err)
	userSvc, err := services.NewUserService(env.DB, auditSvc)
	require.NoError(t, err)

	existingEmail := "team-existing-" + uuid.NewString() + "@example.com"
	existingUsername := "team-existing-" + uuid.NewString()
	existingUser, err := userSvc.Create(context.Background(), services.CreateUserInput{
		Username: existingUsername,
		Email:    existingEmail,
		Password: "ExistingPass123!",
	})
	require.NoError(t, err)

	createPayload := map[string]any{
		"email":   existingEmail,
		"team_id": teamID,
	}
	createResp := env.Request(http.MethodPost, "/api/invites", createPayload, token)
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())

	var createData struct {
		Invite map[string]any `json:"invite"`
		Token  string         `json:"token"`
		Link   string         `json:"link"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, createResp).Data, &createData)
	require.NotEmpty(t, createData.Token)
	require.Equal(t, existingEmail, createData.Invite["email"])

	redeemPayload := map[string]any{
		"token":      createData.Token,
		"username":   existingUser.Username,
		"password":   "DoesNotMatter123!",
		"first_name": existingUser.FirstName,
		"last_name":  existingUser.LastName,
	}
	redeemResp := env.Request(http.MethodPost, "/api/auth/invite/redeem", redeemPayload, "")
	require.Equal(t, http.StatusCreated, redeemResp.Code, redeemResp.Body.String())

	var redeemData struct {
		User    map[string]any `json:"user"`
		Message string         `json:"message"`
	}
	testutil.DecodeInto(t, testutil.DecodeResponse(t, redeemResp).Data, &redeemData)
	require.Equal(t, strings.ToLower(existingEmail), strings.ToLower(redeemData.User["email"].(string)))
	require.Contains(t, strings.ToLower(redeemData.Message), "team access granted")

	memberList := env.Request(http.MethodGet, "/api/teams/"+teamID+"/members", nil, token)
	require.Equal(t, http.StatusOK, memberList.Code, memberList.Body.String())
	var members []map[string]any
	testutil.DecodeInto(t, testutil.DecodeResponse(t, memberList).Data, &members)

	found := false
	for _, member := range members {
		if member["id"] == existingUser.ID {
			found = true
			break
		}
	}
	require.True(t, found, "existing user should be added to the team")

	var count int64
	require.NoError(t, env.DB.Model(&models.User{}).
		Where("LOWER(email) = ?", strings.ToLower(existingEmail)).
		Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSecurityHandler_Audit(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("SecurePassw0rd!")
	login := env.Login(root.Username, "SecurePassw0rd!")

	resp := env.Request(http.MethodGet, "/api/security/audit", nil, login.AccessToken)
	require.Equal(t, http.StatusOK, resp.Code)

	payload := testutil.DecodeResponse(t, resp)
	require.True(t, payload.Success)

	var data map[string]any
	testutil.DecodeInto(t, payload.Data, &data)
	require.Contains(t, data, "summary")
}
