package handlers

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

const connectionResourceType = "connection"

// SessionParticipantHandler exposes endpoints for managing active session participants.
type SessionParticipantHandler struct {
	db        *gorm.DB
	lifecycle *services.SessionLifecycleService
	checker   *permissions.Checker
}

// NewSessionParticipantHandler constructs a participant handler.
func NewSessionParticipantHandler(db *gorm.DB, lifecycle *services.SessionLifecycleService, checker *permissions.Checker) *SessionParticipantHandler {
	return &SessionParticipantHandler{
		db:        db,
		lifecycle: lifecycle,
		checker:   checker,
	}
}

// ListParticipants returns active participants for the session.
func (h *SessionParticipantHandler) ListParticipants(c *gin.Context) {
	if h == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := c.Request.Context()
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session access"))
		}
		return
	}
	if session.ClosedAt != nil {
		response.Error(c, apperrors.NewBadRequest("session is no longer active"))
		return
	}

	record, ok := h.lifecycle.GetActiveSession(sessionID)
	if !ok || record == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	payload := buildParticipantsPayload(record)
	response.Success(c, http.StatusOK, payload)
}

type addParticipantRequest struct {
	UserID               string `json:"user_id"`
	Role                 string `json:"role"`
	AccessMode           string `json:"access_mode"`
	ConsentedToRecording bool   `json:"consented_to_recording"`
}

// AddParticipant invites a user into the active session.
func (h *SessionParticipantHandler) AddParticipant(c *gin.Context) {
	if h == nil || h.lifecycle == nil || h.db == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	actorID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if actorID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	var payload addParticipantRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid participant payload"))
		return
	}
	targetUserID := strings.TrimSpace(payload.UserID)
	if targetUserID == "" {
		response.Error(c, apperrors.NewBadRequest("user id is required"))
		return
	}
	if strings.EqualFold(targetUserID, actorID) {
		// Already part of session (owner or participant). No-op.
	}

	ctx := c.Request.Context()
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session access"))
		}
		return
	}
	if session.ClosedAt != nil {
		response.Error(c, apperrors.NewBadRequest("session is no longer active"))
		return
	}

	allowed, err := h.canManageSharing(ctx, actorID, session.ConnectionID, session.OwnerUserID)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "unable to verify permissions"))
		return
	}
	if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	targetUser, err := h.loadActiveUser(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, apperrors.ErrNotFound)
		} else {
			response.Error(c, apperrors.Wrap(err, "failed to load user"))
		}
		return
	}
	if !targetUser.IsActive {
		response.Error(c, apperrors.NewBadRequest("user is inactive"))
		return
	}

	if err := h.ensureUserHasSessionAccess(ctx, targetUserID, session.ConnectionID); err != nil {
		response.Error(c, err)
		return
	}

	displayName := displayNameForUser(targetUser)
	accessMode := strings.ToLower(strings.TrimSpace(payload.AccessMode))
	if accessMode != "write" {
		accessMode = "read"
	} else if !h.hasGrantPermission(ctx, actorID, session.ConnectionID, session.OwnerUserID) {
		accessMode = "read"
	}

	actor := resolveSessionActor(c.Request.Context(), actorID, session.OwnerUserID)

	participant, err := h.lifecycle.AddParticipant(ctx, services.AddParticipantParams{
		SessionID:            sessionID,
		UserID:               targetUserID,
		UserName:             displayName,
		Role:                 payload.Role,
		AccessMode:           accessMode,
		ConsentedToRecording: payload.ConsentedToRecording,
		GrantedByUserID: func() *string {
			if accessMode != "write" {
				return nil
			}
			id := actorID
			return &id
		}(),
		Actor: actor,
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "failed to add participant"))
		return
	}

	record, ok := h.lifecycle.GetActiveSession(sessionID)
	if !ok || record == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusCreated, buildParticipantDTO(participant, record))
}

// RemoveParticipant removes a participant from the session. Participants can remove themselves.
func (h *SessionParticipantHandler) RemoveParticipant(c *gin.Context) {
	if h == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	userID := strings.TrimSpace(c.Param("userID"))
	if sessionID == "" || userID == "" {
		response.Error(c, apperrors.NewBadRequest("session id and user id are required"))
		return
	}

	actorID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if actorID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := c.Request.Context()
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session access"))
		}
		return
	}

	allowed := strings.EqualFold(actorID, userID)
	if !allowed {
		allowed, err = h.canManageSharing(ctx, actorID, session.ConnectionID, session.OwnerUserID)
		if err != nil {
			response.Error(c, apperrors.Wrap(err, "unable to verify permissions"))
			return
		}
	}
	if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	actor := resolveSessionActor(c.Request.Context(), actorID, session.OwnerUserID)
	removed, err := h.lifecycle.RemoveParticipant(ctx, services.RemoveParticipantParams{
		SessionID: sessionID,
		UserID:    userID,
		Actor:     actor,
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "failed to remove participant"))
		return
	}
	if !removed {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	response.Success(c, http.StatusNoContent, gin.H{"removed": true})
}

// GrantWrite promotes the participant to the sole write holder.
func (h *SessionParticipantHandler) GrantWrite(c *gin.Context) {
	if h == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	userID := strings.TrimSpace(c.Param("userID"))
	if sessionID == "" || userID == "" {
		response.Error(c, apperrors.NewBadRequest("session id and user id are required"))
		return
	}

	actorID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if actorID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := c.Request.Context()
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session access"))
		}
		return
	}

	if !h.hasGrantPermission(ctx, actorID, session.ConnectionID, session.OwnerUserID) {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	actor := resolveSessionActor(c.Request.Context(), actorID, session.OwnerUserID)
	participant, err := h.lifecycle.GrantWriteAccess(ctx, services.GrantWriteParams{
		SessionID:       sessionID,
		UserID:          userID,
		GrantedByUserID: &actorID,
		Actor:           actor,
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "failed to grant write access"))
		return
	}

	record, ok := h.lifecycle.GetActiveSession(sessionID)
	if !ok || record == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, buildParticipantDTO(participant, record))
}

// RelinquishWrite allows a participant (or owner/admin) to release write access.
func (h *SessionParticipantHandler) RelinquishWrite(c *gin.Context) {
	if h == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	userID := strings.TrimSpace(c.Param("userID"))
	if sessionID == "" || userID == "" {
		response.Error(c, apperrors.NewBadRequest("session id and user id are required"))
		return
	}

	actorID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if actorID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := c.Request.Context()
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session access"))
		}
		return
	}

	allowed := strings.EqualFold(actorID, userID)
	if !allowed {
		allowed = h.hasGrantPermission(ctx, actorID, session.ConnectionID, session.OwnerUserID)
	}
	if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	actor := resolveSessionActor(c.Request.Context(), actorID, session.OwnerUserID)
	participant, newWriter, err := h.lifecycle.RelinquishWriteAccess(ctx, services.RelinquishWriteParams{
		SessionID: sessionID,
		UserID:    userID,
		Actor:     actor,
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "failed to relinquish write access"))
		return
	}

	record, ok := h.lifecycle.GetActiveSession(sessionID)
	if !ok || record == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"participant": buildParticipantDTO(participant, record),
		"write_holder": func() any {
			if newWriter == nil {
				return nil
			}
			return buildParticipantDTO(*newWriter, record)
		}(),
	})
}

func (h *SessionParticipantHandler) canManageSharing(ctx context.Context, actorID, connectionID, ownerID string) (bool, error) {
	if strings.EqualFold(actorID, ownerID) {
		return true, nil
	}
	if h.checker == nil {
		return false, nil
	}

	if ok, err := h.checker.CheckResource(ctx, actorID, connectionResourceType, connectionID, "protocol:ssh.share"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	ok, err := h.checker.Check(ctx, actorID, "protocol:ssh.share")
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (h *SessionParticipantHandler) hasGrantPermission(ctx context.Context, actorID, connectionID, ownerID string) bool {
	if strings.EqualFold(actorID, ownerID) {
		return true
	}
	if h.checker == nil {
		return false
	}

	if ok, err := h.checker.CheckResource(ctx, actorID, connectionResourceType, connectionID, "protocol:ssh.grant_write"); err == nil && ok {
		return true
	}

	ok, err := h.checker.Check(ctx, actorID, "protocol:ssh.grant_write")
	if err != nil {
		return false
	}
	return ok
}

func (h *SessionParticipantHandler) ensureUserHasSessionAccess(ctx context.Context, userID, connectionID string) error {
	if h.checker == nil {
		return apperrors.ErrForbidden
	}

	ok, err := h.checker.CheckResource(ctx, userID, connectionResourceType, connectionID, "protocol:ssh.connect")
	if err != nil {
		return apperrors.Wrap(err, "unable to verify connection access")
	}
	if ok {
		return nil
	}

	ok, err = h.checker.Check(ctx, userID, "protocol:ssh.connect")
	if err == nil && ok {
		return nil
	}
	if err != nil {
		return apperrors.Wrap(err, "unable to verify connection access")
	}

	ok, err = h.checker.CheckResource(ctx, userID, connectionResourceType, connectionID, "connection.launch")
	if err != nil {
		return apperrors.Wrap(err, "unable to verify connection access")
	}
	if ok {
		return nil
	}

	ok, err = h.checker.Check(ctx, userID, "connection.launch")
	if err == nil && ok {
		return nil
	}
	if err != nil {
		return apperrors.Wrap(err, "unable to verify connection access")
	}

	return apperrors.ErrForbidden
}

func (h *SessionParticipantHandler) loadActiveUser(ctx context.Context, userID string) (models.User, error) {
	var user models.User
	if err := h.db.WithContext(ctx).
		Select("id", "username", "email", "first_name", "last_name", "is_active").
		First(&user, "id = ?", userID).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

type sessionParticipantDTO struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	UserName      string    `json:"user_name,omitempty"`
	Role          string    `json:"role"`
	AccessMode    string    `json:"access_mode"`
	JoinedAt      time.Time `json:"joined_at"`
	IsOwner       bool      `json:"is_owner"`
	IsWriteHolder bool      `json:"is_write_holder"`
}

type sessionParticipantsResponse struct {
	SessionID     string                  `json:"session_id"`
	ConnectionID  string                  `json:"connection_id"`
	OwnerUserID   string                  `json:"owner_user_id"`
	OwnerUserName string                  `json:"owner_user_name,omitempty"`
	WriteHolder   string                  `json:"write_holder,omitempty"`
	Participants  []sessionParticipantDTO `json:"participants"`
}

func buildParticipantsPayload(record *services.ActiveSessionRecord) sessionParticipantsResponse {
	participants := make([]sessionParticipantDTO, 0, len(record.Participants))
	for _, participant := range record.Participants {
		if participant == nil {
			continue
		}
		participants = append(participants, buildParticipantDTO(*participant, record))
	}
	sort.SliceStable(participants, func(i, j int) bool {
		return participants[i].JoinedAt.Before(participants[j].JoinedAt)
	})

	return sessionParticipantsResponse{
		SessionID:     record.ID,
		ConnectionID:  record.ConnectionID,
		OwnerUserID:   record.OwnerUserID,
		OwnerUserName: record.OwnerUserName,
		WriteHolder:   record.WriteHolder,
		Participants:  participants,
	}
}

func buildParticipantDTO(participant services.ActiveSessionParticipant, record *services.ActiveSessionRecord) sessionParticipantDTO {
	dto := sessionParticipantDTO{
		SessionID:  participant.SessionID,
		UserID:     participant.UserID,
		UserName:   participant.UserName,
		Role:       participant.Role,
		AccessMode: strings.ToLower(participant.AccessMode),
		JoinedAt:   participant.JoinedAt,
	}
	dto.IsOwner = record != nil && strings.EqualFold(record.OwnerUserID, participant.UserID)
	dto.IsWriteHolder = record != nil && strings.EqualFold(record.WriteHolder, participant.UserID)
	return dto
}

func displayNameForUser(user models.User) string {
	fullName := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
	switch {
	case fullName != "":
		return fullName
	case strings.TrimSpace(user.Username) != "":
		return strings.TrimSpace(user.Username)
	case strings.TrimSpace(user.Email) != "":
		return strings.TrimSpace(user.Email)
	default:
		return strings.TrimSpace(user.ID)
	}
}

func resolveSessionActor(ctx context.Context, actorID, ownerID string) services.SessionActor {
	actor, ok := auditctx.FromContext(ctx)
	if !ok {
		return services.SessionActor{
			UserID: actorID,
		}
	}

	username := actor.Username
	if username == "" && strings.EqualFold(actorID, ownerID) {
		username = "owner"
	}

	return services.SessionActor{
		UserID:    actorID,
		Username:  username,
		IPAddress: actor.IPAddress,
		UserAgent: actor.UserAgent,
	}
}
