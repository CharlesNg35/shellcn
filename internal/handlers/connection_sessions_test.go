package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestActiveConnectionHandler_ListActiveScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	require.NoError(t, permissions.Sync(context.Background(), db))

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	activeSvc := services.NewActiveSessionService(nil)

	teamA := &models.Team{BaseModel: models.BaseModel{ID: "team-a"}, Name: "Team A"}
	teamB := &models.Team{BaseModel: models.BaseModel{ID: "team-b"}, Name: "Team B"}
	require.NoError(t, db.Create(teamA).Error)
	require.NoError(t, db.Create(teamB).Error)

	userTeam := &models.User{BaseModel: models.BaseModel{ID: "user-team"}, Username: "user-team", Email: "team@example.com", Password: "password", IsActive: true}
	userAdmin := &models.User{BaseModel: models.BaseModel{ID: "user-admin"}, Username: "user-admin", Email: "admin@example.com", Password: "password", IsActive: true}
	userOther := &models.User{BaseModel: models.BaseModel{ID: "user-other"}, Username: "user-other", Email: "other@example.com", Password: "password", IsActive: true}
	require.NoError(t, db.Create(userTeam).Error)
	require.NoError(t, db.Create(userAdmin).Error)
	require.NoError(t, db.Create(userOther).Error)

	require.NoError(t, db.Model(teamA).Association("Users").Append(userTeam))

	var permConnectionView models.Permission
	var permActiveViewTeam models.Permission
	var permActiveViewAll models.Permission
	require.NoError(t, db.First(&permConnectionView, "id = ?", "connection.view").Error)
	require.NoError(t, db.First(&permActiveViewTeam, "id = ?", "session.active.view_team").Error)
	require.NoError(t, db.First(&permActiveViewAll, "id = ?", "session.active.view_all").Error)

	roleTeam := &models.Role{BaseModel: models.BaseModel{ID: "role.team"}, Name: "Team Viewer"}
	roleAdmin := &models.Role{BaseModel: models.BaseModel{ID: "role.admin"}, Name: "Session Admin"}
	require.NoError(t, db.Create(roleTeam).Error)
	require.NoError(t, db.Create(roleAdmin).Error)
	require.NoError(t, db.Model(roleTeam).Association("Permissions").Append(&permConnectionView, &permActiveViewTeam))
	require.NoError(t, db.Model(roleAdmin).Association("Permissions").Append(&permConnectionView, &permActiveViewAll))

	require.NoError(t, db.Model(userTeam).Association("Roles").Append(roleTeam))
	require.NoError(t, db.Model(userAdmin).Association("Roles").Append(roleAdmin))

	now := time.Now().UTC()

	register := func(record services.ActiveSessionRecord) {
		require.NoError(t, activeSvc.RegisterSession(&record))
	}

	register(services.ActiveSessionRecord{
		ID:           "sess-1",
		ConnectionID: "conn-1",
		UserID:       userTeam.ID,
		ProtocolID:   "ssh",
		StartedAt:    now,
		LastSeenAt:   now,
	})
	register(services.ActiveSessionRecord{
		ID:           "sess-2",
		ConnectionID: "conn-2",
		UserID:       userTeam.ID,
		ProtocolID:   "ssh",
		TeamID:       &teamA.ID,
		StartedAt:    now,
		LastSeenAt:   now,
	})
	register(services.ActiveSessionRecord{
		ID:           "sess-3",
		ConnectionID: "conn-3",
		UserID:       userOther.ID,
		ProtocolID:   "ssh",
		TeamID:       &teamA.ID,
		StartedAt:    now,
		LastSeenAt:   now,
	})
	register(services.ActiveSessionRecord{
		ID:           "sess-4",
		ConnectionID: "conn-4",
		UserID:       userOther.ID,
		ProtocolID:   "ssh",
		TeamID:       &teamB.ID,
		StartedAt:    now,
		LastSeenAt:   now,
	})
	register(services.ActiveSessionRecord{
		ID:           "sess-admin",
		ConnectionID: "conn-admin",
		UserID:       userAdmin.ID,
		ProtocolID:   "ssh",
		StartedAt:    now,
		LastSeenAt:   now,
	})

	handler := NewActiveConnectionHandler(activeSvc, checker)

	perform := func(userID, scope, teamFilter string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, err := http.NewRequest(http.MethodGet, "/connections/active", nil)
		require.NoError(t, err)
		q := req.URL.Query()
		if scope != "" {
			q.Set("scope", scope)
		}
		if teamFilter != "" {
			q.Set("team_id", teamFilter)
		}
		req.URL.RawQuery = q.Encode()
		c.Request = req
		c.Set(middleware.CtxUserIDKey, userID)
		handler.ListActive(c)
		return w
	}

	t.Run("team scope returns personal and team sessions", func(t *testing.T) {
		w := perform(userTeam.ID, "team", "")
		require.Equal(t, http.StatusOK, w.Code)
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		var sessions []map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &sessions))
		require.Len(t, sessions, 3)
	})

	t.Run("team scope filtered to specific team", func(t *testing.T) {
		w := perform(userTeam.ID, "team", teamA.ID)
		require.Equal(t, http.StatusOK, w.Code)
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		var sessions []map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &sessions))
		require.Len(t, sessions, 2)
		for _, session := range sessions {
			if value, ok := session["team_id"].(string); ok {
				require.Equal(t, teamA.ID, value)
			}
		}
	})

	t.Run("team scope forbidden for other team", func(t *testing.T) {
		w := perform(userTeam.ID, "team", teamB.ID)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("admin all scope returns every session", func(t *testing.T) {
		w := perform(userAdmin.ID, "all", "")
		require.Equal(t, http.StatusOK, w.Code)
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		var sessions []map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &sessions))
		require.Len(t, sessions, 5)
	})

	t.Run("admin personal scope limits results", func(t *testing.T) {
		w := perform(userAdmin.ID, "personal", "")
		require.Equal(t, http.StatusOK, w.Code)
		var envelope apiEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		var sessions []map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &sessions))
		require.Len(t, sessions, 1)
		require.Equal(t, "sess-admin", sessions[0]["id"])
	})
}
