package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
)

func TestSessionParticipantHandler_Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	active := services.NewActiveSessionService(nil)
	lifecycle, err := services.NewSessionLifecycleService(
		db,
		active,
		services.WithSessionAuditService(auditSvc),
		services.WithLifecycleClock(func() time.Time { return time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC) }),
	)
	require.NoError(t, err)

	handler := NewSessionParticipantHandler(db, lifecycle, checker)

	owner := createTestUser(t, db, "owner-user")
	participant := createTestUser(t, db, "participant-user")
	connection := createTestConnection(t, db, owner.ID)

	// Grant protocol connect permission to participant.
	require.NoError(t, db.Create(&models.ResourcePermission{
		ResourceType:  "connection",
		ResourceID:    connection.ID,
		PrincipalType: "user",
		PrincipalID:   participant.ID,
		PermissionID:  "connection.view",
	}).Error)

	require.NoError(t, db.Create(&models.ResourcePermission{
		ResourceType:  "connection",
		ResourceID:    connection.ID,
		PrincipalType: "user",
		PrincipalID:   participant.ID,
		PermissionID:  "connection.launch",
	}).Error)

	require.NoError(t, db.Create(&models.ResourcePermission{
		ResourceType:  "connection",
		ResourceID:    connection.ID,
		PrincipalType: "user",
		PrincipalID:   participant.ID,
		PermissionID:  "protocol:ssh.connect",
	}).Error)

	session, err := lifecycle.StartSession(
		contextWithActor(owner.ID, owner.Username),
		services.StartSessionParams{
			SessionID:      "sess-participants",
			ConnectionID:   connection.ID,
			ConnectionName: connection.Name,
			ProtocolID:     "ssh",
			OwnerUserID:    owner.ID,
			OwnerUserName:  owner.Username,
		},
	)
	require.NoError(t, err)

	middlewareChain := func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, owner.ID)
		ctx := auditctx.WithActor(c.Request.Context(), auditctx.Actor{
			UserID:    owner.ID,
			Username:  owner.Username,
			IPAddress: "127.0.0.1",
			UserAgent: "test-suite",
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

	router.POST("/api/active-sessions/:sessionID/participants", middlewareChain, handler.AddParticipant)
	router.POST("/api/active-sessions/:sessionID/participants/:userID/write", middlewareChain, handler.GrantWrite)
	router.DELETE("/api/active-sessions/:sessionID/participants/:userID/write", middlewareChain, handler.RelinquishWrite)
	router.DELETE("/api/active-sessions/:sessionID/participants/:userID", middlewareChain, handler.RemoveParticipant)
	router.GET("/api/active-sessions/:sessionID/participants", middlewareChain, handler.ListParticipants)

	// Invite participant
	payload, _ := json.Marshal(map[string]any{
		"user_id": participant.ID,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/active-sessions/"+session.ID+"/participants", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var envelope apiEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)

	var participantDTO sessionParticipantDTO
	require.NoError(t, json.Unmarshal(envelope.Data, &participantDTO))
	require.Equal(t, participant.ID, participantDTO.UserID)
	require.Equal(t, "read", participantDTO.AccessMode)
	require.False(t, participantDTO.IsWriteHolder)

	// Grant write access
	grantReq, _ := http.NewRequest(http.MethodPost, "/api/active-sessions/"+session.ID+"/participants/"+participant.ID+"/write", nil)
	grantRec := httptest.NewRecorder()
	router.ServeHTTP(grantRec, grantReq)
	require.Equal(t, http.StatusOK, grantRec.Code, grantRec.Body.String())

	require.NoError(t, json.Unmarshal(grantRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.NoError(t, json.Unmarshal(envelope.Data, &participantDTO))
	require.Equal(t, "write", participantDTO.AccessMode)
	require.True(t, participantDTO.IsWriteHolder)

	// Relinquish write access (owner invokes)
	releaseReq, _ := http.NewRequest(http.MethodDelete, "/api/active-sessions/"+session.ID+"/participants/"+participant.ID+"/write", nil)
	releaseRec := httptest.NewRecorder()
	router.ServeHTTP(releaseRec, releaseReq)
	require.Equal(t, http.StatusOK, releaseRec.Code, releaseRec.Body.String())

	require.NoError(t, json.Unmarshal(releaseRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	var releasePayload struct {
		Participant sessionParticipantDTO  `json:"participant"`
		WriteHolder *sessionParticipantDTO `json:"write_holder"`
	}
	require.NoError(t, json.Unmarshal(envelope.Data, &releasePayload))
	require.Equal(t, "read", releasePayload.Participant.AccessMode)
	require.NotNil(t, releasePayload.WriteHolder)
	require.Equal(t, owner.ID, releasePayload.WriteHolder.UserID)
	require.Equal(t, "write", releasePayload.WriteHolder.AccessMode)

	// Remove participant
	removeReq, _ := http.NewRequest(http.MethodDelete, "/api/active-sessions/"+session.ID+"/participants/"+participant.ID, nil)
	removeRec := httptest.NewRecorder()
	router.ServeHTTP(removeRec, removeReq)
	require.Equal(t, http.StatusNoContent, removeRec.Code, removeRec.Body.String())

	// Participants list now only owner remains
	listReq, _ := http.NewRequest(http.MethodGet, "/api/active-sessions/"+session.ID+"/participants", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	var listPayload sessionParticipantsResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &listPayload))
	require.Len(t, listPayload.Participants, 1)
	require.Equal(t, owner.ID, listPayload.Participants[0].UserID)
}
