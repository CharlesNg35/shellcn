package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/handlers/testutil"
)

func TestMonitoringSummaryRequiresPermission(t *testing.T) {
	t.Parallel()

	env := testutil.NewEnv(t)
	root := env.CreateRootUser("Secret123!")
	login := env.Login(root.Username, "Secret123!")

	// unauthenticated request should be rejected
	resp := env.Request(http.MethodGet, "/api/monitoring/summary", nil, "")
	require.Equal(t, http.StatusUnauthorized, resp.Code, resp.Body.String())

	// root user has monitoring.view permission
	resp = env.Request(http.MethodGet, "/api/monitoring/summary", nil, login.AccessToken)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}
