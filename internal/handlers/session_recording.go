package handlers

import (
	"context"
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

// List returns paginated recording summaries available to the caller.
func (h *SessionRecordingHandler) List(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			response.Error(c, apperrors.NewBadRequest("invalid page parameter"))
			return
		}
		page = value
	}

	perPage := 25
	if raw := strings.TrimSpace(c.Query("per_page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			response.Error(c, apperrors.NewBadRequest("invalid per_page parameter"))
			return
		}
		if value > 100 {
			value = 100
		}
		perPage = value
	}
	offset := (page - 1) * perPage

	scopeValue := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	scope := services.RecordingScopePersonal
	switch scopeValue {
	case "", "personal", "self", "me":
		scope = services.RecordingScopePersonal
	case "team", "teams":
		scope = services.RecordingScopeTeam
	case "all", "global":
		scope = services.RecordingScopeAll
	default:
		response.Error(c, apperrors.NewBadRequest("invalid scope value"))
		return
	}

	teamFilter := strings.TrimSpace(c.Query("team_id"))
	protocolFilter := strings.TrimSpace(c.Query("protocol_id"))
	connectionFilter := strings.TrimSpace(c.Query("connection_id"))
	sessionFilter := strings.TrimSpace(c.Query("session_id"))
	ownerFilter := strings.TrimSpace(c.Query("owner_user_id"))
	createdByFilter := strings.TrimSpace(c.Query("created_by_user_id"))
	sort := strings.TrimSpace(c.Query("sort"))

	opts := services.ListRecordingsOptions{
		Scope:           scope,
		UserID:          userID,
		Limit:           perPage,
		Offset:          offset,
		Sort:            sort,
		ProtocolID:      protocolFilter,
		ConnectionID:    connectionFilter,
		SessionID:       sessionFilter,
		OwnerUserID:     ownerFilter,
		CreatedByUserID: createdByFilter,
	}

	checkPermission := func(permissionID string) (bool, error) {
		if h.checker == nil || strings.TrimSpace(permissionID) == "" {
			return false, nil
		}
		return h.checker.Check(ctx, userID, permissionID)
	}

	switch scope {
	case services.RecordingScopeAll:
		allowed := false
		if ok, err := checkPermission("session.recording.view_all"); err != nil {
			response.Error(c, err)
			return
		} else if ok {
			allowed = true
		}
		if !allowed {
			for _, permID := range []string{"permission.manage", "connection.manage"} {
				ok, err := checkPermission(permID)
				if err != nil {
					response.Error(c, err)
					return
				}
				if ok {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		if teamFilter != "" && !strings.EqualFold(teamFilter, "personal") {
			opts.TeamID = teamFilter
		}
	case services.RecordingScopeTeam:
		hasAll, err := checkPermission("session.recording.view_all")
		if err != nil {
			response.Error(c, err)
			return
		}
		if hasAll {
			opts.Scope = services.RecordingScopeAll
			if strings.EqualFold(teamFilter, "personal") {
				opts.Scope = services.RecordingScopePersonal
			} else if teamFilter != "" {
				opts.TeamID = teamFilter
			}
			break
		}

		hasTeam, err := checkPermission("session.recording.view_team")
		if err != nil {
			response.Error(c, err)
			return
		}
		allowed := hasTeam
		if !allowed {
			for _, permID := range []string{"permission.manage", "connection.manage"} {
				ok, err := checkPermission(permID)
				if err != nil {
					response.Error(c, err)
					return
				}
				if ok {
					allowed = true
					opts.Scope = services.RecordingScopeAll
					break
				}
			}
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		if opts.Scope == services.RecordingScopeAll {
			if strings.EqualFold(teamFilter, "personal") {
				opts.Scope = services.RecordingScopePersonal
			} else if teamFilter != "" {
				opts.TeamID = teamFilter
			}
			break
		}
		if strings.EqualFold(teamFilter, "personal") {
			opts.Scope = services.RecordingScopePersonal
			break
		}
		if h.checker == nil {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		teamIDs, err := h.checker.GetUserTeamIDs(ctx, userID)
		if err != nil {
			response.Error(c, err)
			return
		}
		if len(teamIDs) == 0 {
			response.SuccessWithMeta(c, http.StatusOK, []recordingSummaryDTO{}, &response.Meta{
				Page: page, PerPage: perPage, Total: 0, TotalPages: 0,
			})
			return
		}
		opts.TeamIDs = teamIDs
		if teamFilter != "" {
			matched := ""
			for _, id := range teamIDs {
				if strings.EqualFold(strings.TrimSpace(id), teamFilter) {
					matched = id
					break
				}
			}
			if matched == "" {
				response.Error(c, apperrors.ErrForbidden)
				return
			}
			opts.TeamID = matched
		}
	default:
		if teamFilter != "" && !strings.EqualFold(teamFilter, "personal") {
			response.Error(c, apperrors.NewBadRequest("team_id filter requires team scope"))
			return
		}
	}

	records, total, err := h.recorder.ListRecordings(ctx, opts)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "list recordings"))
		return
	}

	items := make([]recordingSummaryDTO, 0, len(records))
	for _, record := range records {
		items = append(items, toRecordingSummaryDTO(record))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	meta := &response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      int(total),
		TotalPages: totalPages,
	}

	response.SuccessWithMeta(c, http.StatusOK, items, meta)
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

// Delete removes a stored session recording.
func (h *SessionRecordingHandler) Delete(c *gin.Context) {
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

	record, session, err := h.recorder.GetRecord(ctx, recordID)
	if err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			response.Error(c, apperrors.ErrNotFound)
		} else {
			response.Error(c, apperrors.Wrap(err, "fetch recording"))
		}
		return
	}

	hasDelete := false
	if h.checker != nil {
		if ok, permErr := h.checker.Check(ctx, userID, "session.recording.delete"); permErr != nil {
			response.Error(c, permErr)
			return
		} else if ok {
			hasDelete = true
		} else {
			for _, permID := range []string{"permission.manage", "connection.manage"} {
				ok, permErr := h.checker.Check(ctx, userID, permID)
				if permErr != nil {
					response.Error(c, permErr)
					return
				}
				if ok {
					hasDelete = true
					break
				}
			}
		}
	}

	if !hasDelete {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	allowed, err := h.canAccessRecording(ctx, record, session, userID)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "authorise recording access"))
		return
	}
	if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	if err := h.recorder.DeleteRecording(ctx, recordID); err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			response.Error(c, apperrors.ErrNotFound)
		} else {
			response.Error(c, apperrors.Wrap(err, "delete recording"))
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"record_id": recordID,
		"deleted":   true,
	})
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
	record, session, err := h.recorder.GetRecord(ctx, recordID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		default:
			response.Error(c, apperrors.Wrap(err, "fetch recording"))
		}
		return
	}

	allowed, err := h.canAccessRecording(ctx, record, session, userID)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "authorise recording access"))
		return
	}
	if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	reader, openedRecord, err := h.recorder.OpenRecording(ctx, recordID)
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
	record = openedRecord

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

type recordingSummaryDTO struct {
	ID                string     `json:"record_id"`
	SessionID         string     `json:"session_id"`
	ConnectionID      string     `json:"connection_id"`
	ConnectionName    string     `json:"connection_name,omitempty"`
	ProtocolID        string     `json:"protocol_id"`
	OwnerUserID       string     `json:"owner_user_id"`
	OwnerUserName     string     `json:"owner_user_name,omitempty"`
	TeamID            *string    `json:"team_id,omitempty"`
	CreatedByUserID   string     `json:"created_by_user_id"`
	CreatedByUserName string     `json:"created_by_user_name,omitempty"`
	StorageKind       string     `json:"storage_kind"`
	StoragePath       string     `json:"storage_path"`
	SizeBytes         int64      `json:"size_bytes"`
	DurationSeconds   int64      `json:"duration_seconds"`
	Checksum          string     `json:"checksum,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	RetentionUntil    *time.Time `json:"retention_until,omitempty"`
}

func toRecordingSummaryDTO(summary services.RecordingSummary) recordingSummaryDTO {
	return recordingSummaryDTO{
		ID:                summary.RecordID,
		SessionID:         summary.SessionID,
		ConnectionID:      summary.ConnectionID,
		ConnectionName:    summary.ConnectionName,
		ProtocolID:        summary.ProtocolID,
		OwnerUserID:       summary.OwnerUserID,
		OwnerUserName:     summary.OwnerUserName,
		TeamID:            summary.TeamID,
		CreatedByUserID:   summary.CreatedByUserID,
		CreatedByUserName: summary.CreatedByUserName,
		StorageKind:       summary.StorageKind,
		StoragePath:       summary.StoragePath,
		SizeBytes:         summary.SizeBytes,
		DurationSeconds:   summary.DurationSeconds,
		Checksum:          summary.Checksum,
		CreatedAt:         summary.CreatedAt,
		RetentionUntil:    summary.RetentionUntil,
	}
}

func (h *SessionRecordingHandler) canAccessRecording(ctx context.Context, record models.ConnectionSessionRecord, session models.ConnectionSession, userID string) (bool, error) {
	if strings.EqualFold(session.OwnerUserID, userID) || strings.EqualFold(record.CreatedByUserID, userID) {
		return true, nil
	}

	if h.lifecycle != nil {
		if _, err := h.lifecycle.AuthorizeSessionAccess(ctx, session.ID, userID); err == nil {
			return true, nil
		} else if err != nil && !errors.Is(err, services.ErrSessionAccessDenied) && !errors.Is(err, services.ErrSessionNotFound) {
			return false, err
		}
	}

	if h.checker == nil {
		return false, nil
	}

	resourcePerm := recordingPermissionForProtocol(session.ProtocolID)
	if ok, err := h.checker.CheckResource(ctx, userID, "connection", session.ConnectionID, resourcePerm); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	if ok, err := h.checker.Check(ctx, userID, "session.recording.view_all"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	if session.TeamID != nil && strings.TrimSpace(*session.TeamID) != "" {
		if ok, err := h.checker.Check(ctx, userID, "session.recording.view_team"); err != nil {
			return false, err
		} else if ok {
			teamIDs, err := h.checker.GetUserTeamIDs(ctx, userID)
			if err != nil {
				return false, err
			}
			for _, teamID := range teamIDs {
				if strings.EqualFold(strings.TrimSpace(teamID), strings.TrimSpace(*session.TeamID)) {
					return true, nil
				}
			}
		}
	}

	for _, permID := range []string{"permission.manage", "connection.manage"} {
		ok, err := h.checker.Check(ctx, userID, permID)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
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
