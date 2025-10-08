package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/handlers/testutil"
)

func TestAuthHandler_LoginRefreshLogout(t *testing.T) {
	env := testutil.NewEnv(t)
	root := env.CreateRootUser("AuthPassw0rd!")

	login := env.Login(root.Username, "AuthPassw0rd!")
	token := login.AccessToken

	me := env.Request(http.MethodGet, "/api/auth/me", nil, token)
	require.Equal(t, http.StatusOK, me.Code)
	meResp := testutil.DecodeResponse(t, me)
	require.True(t, meResp.Success)
	var meData map[string]any
	testutil.DecodeInto(t, meResp.Data, &meData)
	require.Equal(t, login.User.ID, meData["id"])
	require.Equal(t, login.User.Email, meData["email"])

	refreshPayload := map[string]string{"refresh_token": login.RefreshToken}
	refresh := env.Request(http.MethodPost, "/api/auth/refresh", refreshPayload, "")
	require.Equal(t, http.StatusOK, refresh.Code, refresh.Body.String())
	var refreshed testutil.TokenPair
	testutil.DecodeInto(t, testutil.DecodeResponse(t, refresh).Data, &refreshed)
	require.NotEqual(t, "", refreshed.AccessToken)
	require.NotEqual(t, "", refreshed.RefreshToken)
	require.Greater(t, refreshed.ExpiresIn, 0)

	logout := env.Request(http.MethodPost, "/api/auth/logout", nil, token)
	require.Equal(t, http.StatusOK, logout.Code)

	unauth := env.Request(http.MethodGet, "/api/auth/me", nil, "")
	require.Equal(t, http.StatusUnauthorized, unauth.Code)
}

func TestAuthHandler_LoginValidation(t *testing.T) {
	env := testutil.NewEnv(t)

	payload := map[string]any{
		"identifier": " ",
		"password":   "",
	}

	resp := env.Request(http.MethodPost, "/api/auth/login", payload, "")
	require.Equal(t, http.StatusBadRequest, resp.Code)
	decoded := testutil.DecodeResponse(t, resp)
	require.False(t, decoded.Success)
	require.NotNil(t, decoded.Error)
	require.Equal(t, "BAD_REQUEST", decoded.Error.Code)
}
