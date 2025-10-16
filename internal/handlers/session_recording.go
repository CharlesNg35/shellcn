package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// SessionRecordingHandler exposes session recording lifecycle endpoints.
type SessionRecordingHandler struct {
	recorder  *services.RecorderService
	lifecycle *services.SessionLifecycleService
	checker   *permissions.Checker
}

// NewSessionRecordingHandler constructs a handler once dependencies are provided.
func NewSessionRecordingHandler(recorder *services.RecorderService, lifecycle *services.SessionLifecycleService, checker *permissions.Checker) *SessionRecordingHandler {
	return &SessionRecordingHandler{
		recorder:  recorder,
		lifecycle: lifecycle,
		checker:   checker,
	}
}

// Status reports the recording state for the supplied session.
func (h *SessionRecordingHandler) Status(c *gin.Context) {
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

	ctx := requestContext(c)
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, userID)
	if err != nil {
		h.handleLifecycleError(c, err)
		return
	}

	status, err := h.recorder.Status(ctx, sessionID)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "fetch recording status"))
		return
	}

	payload := buildRecordingStatusDTO(status, session)
	response.Success(c, http.StatusOK, payload)
}

// Stop halts the active recording for the session when permitted.
func (h *SessionRecordingHandler) Stop(c *gin.Context) {
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

	ctx := requestContext(c)
	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, userID)
	if err != nil {
		h.handleLifecycleError(c, err)
		return
	}

	if !strings.EqualFold(session.OwnerUserID, userID) {
		permissionID := recordingPermissionForProtocol(session.ProtocolID)
		allowed, permErr := h.checker.CheckResource(ctx, userID, "connection", session.ConnectionID, permissionID)
		if permErr != nil {
			response.Error(c, permErr)
			return
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	}

	record, err := h.recorder.StopRecording(ctx, sessionID, userID, "manual_stop")
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "stop recording"))
		return
	}
	if record == nil {
		response.Success(c, http.StatusOK, gin.H{"active": false})
		return
	}

	createdAt := record.CreatedAt
	dto := recordingRecordDTO{
		ID:              record.ID,
		SessionID:       record.SessionID,
		StoragePath:     record.StoragePath,
		StorageKind:     record.StorageKind,
		SizeBytes:       record.SizeBytes,
		DurationSeconds: record.DurationSeconds,
		Checksum:        record.Checksum,
		CreatedAt:       &createdAt,
		RetentionUntil:  record.RetentionUntil,
	}
	response.Success(c, http.StatusOK, dto)
}

// Download streams the stored recording to the client.
func (h *SessionRecordingHandler) Download(c *gin.Context) {
	recordID := strings.TrimSpace(c.Param("recordID"))
	if recordID == "" {
		response.Error(c, apperrors.NewBadRequest("record id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	reader, record, err := h.recorder.OpenRecording(ctx, recordID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		default:
			response.Error(c, apperrors.Wrap(err, "open recording"))
		}
		return
	}
	defer reader.Close()

	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, record.SessionID, userID)
	if err != nil {
		reader.Close()
		h.handleLifecycleError(c, err)
		return
	}

	if !strings.EqualFold(session.OwnerUserID, userID) {
		permissionID := recordingPermissionForProtocol(session.ProtocolID)
		allowed, permErr := h.checker.CheckResource(ctx, userID, "connection", session.ConnectionID, permissionID)
		if permErr != nil {
			response.Error(c, permErr)
			return
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	}

	filename := fmt.Sprintf("%s-%s.cast.gz", record.SessionID, record.ID)

	c.Writer.Header().Set("Content-Type", "application/gzip")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(record.SizeBytes, 10))
	c.Writer.WriteHeader(http.StatusOK)

	if _, err := io.Copy(c.Writer, reader); err != nil {
		c.Error(err) // best effort logging
	}
}

func (h *SessionRecordingHandler) handleLifecycleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrSessionNotFound):
		response.Error(c, apperrors.ErrNotFound)
	case errors.Is(err, services.ErrSessionAccessDenied):
		response.Error(c, apperrors.ErrForbidden)
	default:
		response.Error(c, apperrors.Wrap(err, "session access"))
	}
}

func recordingPermissionForProtocol(protocolID string) string {
	protocolID = strings.ToLower(strings.TrimSpace(protocolID))
	if protocolID == "" {
		protocolID = "ssh"
	}
	return fmt.Sprintf("protocol:%s.record", protocolID)
}

type recordingRecordDTO struct {
	ID              string     `json:"record_id"`
	SessionID       string     `json:"session_id"`
	StoragePath     string     `json:"storage_path"`
	StorageKind     string     `json:"storage_kind"`
	SizeBytes       int64      `json:"size_bytes"`
	DurationSeconds int64      `json:"duration_seconds"`
	Checksum        string     `json:"checksum,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	RetentionUntil  *time.Time `json:"retention_until,omitempty"`
}

type recordingStatusDTO struct {
	Active        bool                `json:"active"`
	SessionID     string              `json:"session_id"`
	StartedAt     *time.Time          `json:"started_at,omitempty"`
	LastEventAt   *time.Time          `json:"last_event_at,omitempty"`
	BytesRecorded int64               `json:"bytes_recorded"`
	Record        *recordingRecordDTO `json:"record,omitempty"`
	RecordingMode string              `json:"recording_mode"`
}

func buildRecordingStatusDTO(status services.RecordingStatus, session *models.ConnectionSession) recordingStatusDTO {
	dto := recordingStatusDTO{
		Active:        status.Active,
		SessionID:     status.SessionID,
		BytesRecorded: status.BytesRecorded,
	}
	if !status.StartedAt.IsZero() {
		start := status.StartedAt
		dto.StartedAt = &start
	}
	if !status.LastEventAt.IsZero() {
		last := status.LastEventAt
		dto.LastEventAt = &last
	}
	if !status.Active && status.RecordID != "" {
		var createdAt *time.Time
		switch {
		case status.CompletedAt != nil:
			created := *status.CompletedAt
			createdAt = &created
		case !status.LastEventAt.IsZero():
			last := status.LastEventAt
			createdAt = &last
		case !status.StartedAt.IsZero():
			start := status.StartedAt
			createdAt = &start
		}

		dto.Record = &recordingRecordDTO{
			ID:              status.RecordID,
			SessionID:       status.SessionID,
			StoragePath:     status.StoragePath,
			StorageKind:     status.StorageKind,
			SizeBytes:       status.BytesRecorded,
			DurationSeconds: status.Duration,
			Checksum:        status.Checksum,
			CreatedAt:       createdAt,
			RetentionUntil:  status.RetentionUntil,
		}
	}
	dto.RecordingMode = resolveRecordingMode(session, status)
	return dto
}

func resolveRecordingMode(session *models.ConnectionSession, status services.RecordingStatus) string {
	if session != nil && len(session.Metadata) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(session.Metadata, &meta); err == nil {
			if recRaw, ok := meta["recording"]; ok {
				if recMap, ok := recRaw.(map[string]any); ok {
					if modeRaw, ok := recMap["mode"]; ok {
						if mode := strings.TrimSpace(fmt.Sprint(modeRaw)); mode != "" {
							return strings.ToLower(mode)
						}
					}
					if activeRaw, ok := recMap["active"]; ok {
						if active, ok := recordingBool(activeRaw); ok && active {
							return "active"
						}
					}
					if requestedRaw, ok := recMap["requested"]; ok {
						if requested, ok := recordingBool(requestedRaw); ok && requested {
							if status.RecordID != "" {
								return "recorded"
							}
							if status.Active {
								return "active"
							}
							return "enabled"
						}
					}
				}
			}
			if modeRaw, ok := meta["recording_mode"]; ok {
				if mode := strings.TrimSpace(fmt.Sprint(modeRaw)); mode != "" {
					return strings.ToLower(mode)
				}
			}
			if enabledRaw, ok := meta["recording_enabled"]; ok {
				if enabled, ok := recordingBool(enabledRaw); ok && enabled {
					policy := strings.ToLower(strings.TrimSpace(status.PolicyMode))
					if policy == services.RecordingModeForced || policy == services.RecordingModeDisabled {
						return policy
					}
					if status.Active {
						return "active"
					}
					if status.RecordID != "" {
						return "recorded"
					}
					return "enabled"
				}
			}
		}
	}

	if mode := strings.TrimSpace(status.PolicyMode); mode != "" {
		return strings.ToLower(mode)
	}
	if status.Active {
		return "active"
	}
	if status.RecordID != "" {
		return "recorded"
	}
	if status.RetentionUntil != nil {
		return "retained"
	}
	return "inactive"
}

func recordingBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return false, false
}
