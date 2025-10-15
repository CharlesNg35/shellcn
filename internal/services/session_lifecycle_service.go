package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
)

const (
	// SessionStatusPending represents a session that is being prepared.
	SessionStatusPending = "pending"
	// SessionStatusActive indicates the session is currently running.
	SessionStatusActive = "active"
	// SessionStatusClosed marks a session that completed gracefully.
	SessionStatusClosed = "closed"
	// SessionStatusFailed marks a session that terminated with an error.
	SessionStatusFailed = "failed"
)

// SessionRecorder exposes hooks for recorder lifecycle integration.
type SessionRecorder interface {
	OnSessionStarted(ctx context.Context, session *models.ConnectionSession) error
	OnSessionClosed(ctx context.Context, session *models.ConnectionSession, reason string) error
}

// SessionChatStore persists buffered chat messages when sessions close.
type SessionChatStore interface {
	PersistMessages(ctx context.Context, sessionID string, messages []ActiveSessionChatMessage) error
}

// SessionActor captures the actor details for audit logging.
type SessionActor struct {
	UserID    string
	Username  string
	IPAddress string
	UserAgent string
}

// StartSessionParams carries the attributes required to start a session lifecycle.
type StartSessionParams struct {
	SessionID       string
	ConnectionID    string
	ConnectionName  string
	ProtocolID      string
	OwnerUserID     string
	OwnerUserName   string
	TeamID          *string
	Host            string
	Port            int
	Metadata        map[string]any
	ConcurrentLimit int
	StartedAt       time.Time
	Actor           SessionActor
}

// CloseSessionParams controls how sessions are closed or failed.
type CloseSessionParams struct {
	SessionID string
	Status    string
	Reason    string
	EndedAt   time.Time
	Actor     SessionActor
}

// AddParticipantParams describes the participant being added to a session.
type AddParticipantParams struct {
	SessionID            string
	UserID               string
	UserName             string
	Role                 string
	AccessMode           string
	GrantedByUserID      *string
	ConsentedToRecording bool
	JoinedAt             time.Time
	Actor                SessionActor
}

// RemoveParticipantParams contains data for removing a participant from a session.
type RemoveParticipantParams struct {
	SessionID string
	UserID    string
	LeftAt    time.Time
	Actor     SessionActor
}

// GrantWriteParams controls write delegation within an active session.
type GrantWriteParams struct {
	SessionID       string
	UserID          string
	GrantedByUserID *string
	Actor           SessionActor
}

// RelinquishWriteParams controls relinquishing write access.
type RelinquishWriteParams struct {
	SessionID string
	UserID    string
	Actor     SessionActor
}

var (
	// ErrSessionNotFound indicates the session record could not be located.
	ErrSessionNotFound = errors.New("session lifecycle service: session not found")
	// ErrSessionAccessDenied indicates the caller cannot access the session.
	ErrSessionAccessDenied = errors.New("session lifecycle service: access denied")
)

// SessionLifecycleService coordinates persisted session lifecycle with the active service.
type SessionLifecycleService struct {
	db       *gorm.DB
	active   *ActiveSessionService
	audit    *AuditService
	chat     SessionChatStore
	recorder SessionRecorder
	timeNow  func() time.Time
}

// SessionLifecycleOption customises service dependencies.
type SessionLifecycleOption func(*SessionLifecycleService)

// WithSessionAuditService wires the audit service for lifecycle events.
func WithSessionAuditService(audit *AuditService) SessionLifecycleOption {
	return func(s *SessionLifecycleService) {
		s.audit = audit
	}
}

// WithSessionChatStore provides the chat store for message persistence.
func WithSessionChatStore(chat SessionChatStore) SessionLifecycleOption {
	return func(s *SessionLifecycleService) {
		s.chat = chat
	}
}

// WithSessionRecorder supplies recorder hooks.
func WithSessionRecorder(rec SessionRecorder) SessionLifecycleOption {
	return func(s *SessionLifecycleService) {
		s.recorder = rec
	}
}

// WithLifecycleClock overrides the clock used for timestamps (test helper).
func WithLifecycleClock(clock func() time.Time) SessionLifecycleOption {
	return func(s *SessionLifecycleService) {
		if clock != nil {
			s.timeNow = clock
		}
	}
}

// NewSessionLifecycleService constructs the lifecycle service once dependencies are supplied.
func NewSessionLifecycleService(db *gorm.DB, active *ActiveSessionService, opts ...SessionLifecycleOption) (*SessionLifecycleService, error) {
	if db == nil {
		return nil, errors.New("session lifecycle service: db is required")
	}
	if active == nil {
		return nil, errors.New("session lifecycle service: active session service is required")
	}

	svc := &SessionLifecycleService{
		db:      db,
		active:  active,
		timeNow: time.Now,
	}
	for _, opt := range opts {
		opt(svc)
	}

	return svc, nil
}

// StartSession persists the lifecycle record, registers the active session and emits audit + metrics.
func (s *SessionLifecycleService) StartSession(ctx context.Context, params StartSessionParams) (*models.ConnectionSession, error) {
	if s == nil {
		return nil, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)

	if strings.TrimSpace(params.ConnectionID) == "" {
		return nil, errors.New("session lifecycle service: connection id is required")
	}
	if strings.TrimSpace(params.ProtocolID) == "" {
		return nil, errors.New("session lifecycle service: protocol id is required")
	}
	ownerID := strings.TrimSpace(params.OwnerUserID)
	if ownerID == "" {
		return nil, errors.New("session lifecycle service: owner user id is required")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	startedAt := params.StartedAt
	if startedAt.IsZero() {
		startedAt = s.timeNow()
	}

	metadata := cloneMetadata(params.Metadata)
	if params.Host != "" {
		metadata["host"] = params.Host
	}
	if params.Port > 0 {
		metadata["port"] = params.Port
	}
	metaJSON, err := encodeMetadata(metadata)
	if err != nil {
		return nil, err
	}

	session := models.ConnectionSession{
		BaseModel:       models.BaseModel{ID: sessionID},
		ConnectionID:    strings.TrimSpace(params.ConnectionID),
		ProtocolID:      strings.TrimSpace(params.ProtocolID),
		OwnerUserID:     ownerID,
		TeamID:          params.TeamID,
		Status:          SessionStatusActive,
		StartedAt:       startedAt,
		LastHeartbeatAt: startedAt,
		Metadata:        metaJSON,
	}

	activeRecord := &ActiveSessionRecord{
		ID:              sessionID,
		ConnectionID:    session.ConnectionID,
		ConnectionName:  params.ConnectionName,
		UserID:          ownerID,
		UserName:        strings.TrimSpace(params.OwnerUserName),
		TeamID:          params.TeamID,
		ProtocolID:      session.ProtocolID,
		StartedAt:       startedAt,
		LastSeenAt:      startedAt,
		Host:            params.Host,
		Port:            params.Port,
		Metadata:        metadata,
		ConcurrentLimit: params.ConcurrentLimit,
	}

	registered := false
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&session).Error; err != nil {
			return err
		}

		participant := models.ConnectionSessionParticipant{
			SessionID:  session.ID,
			UserID:     ownerID,
			Role:       "owner",
			AccessMode: "write",
			JoinedAt:   startedAt,
		}
		if err := tx.
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&participant).Error; err != nil {
			return err
		}

		if err := s.active.RegisterSession(activeRecord); err != nil {
			return err
		}
		registered = true
		return nil
	}); err != nil {
		if registered {
			s.active.UnregisterSession(sessionID)
		}
		return nil, err
	}

	monitoring.AdjustActiveSessions(1)

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.started",
			Resource: fmt.Sprintf("connection:%s", session.ConnectionID),
			Result:   "success",
			Metadata: map[string]any{
				"session_id":  session.ID,
				"protocol_id": session.ProtocolID,
				"host":        params.Host,
				"port":        params.Port,
			},
		}, ownerID)); err != nil {
			return nil, err
		}
	}

	if s.recorder != nil {
		if err := s.recorder.OnSessionStarted(ctx, &session); err != nil {
			return nil, err
		}
	}

	return &session, nil
}

// Heartbeat updates the persisted heartbeat timestamp and touches the active registry.
func (s *SessionLifecycleService) Heartbeat(ctx context.Context, sessionID string) error {
	if s == nil {
		return errors.New("session lifecycle service: service not initialised")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session lifecycle service: session id is required")
	}
	ctx = ensureContext(ctx)

	now := s.timeNow()
	if err := s.db.WithContext(ctx).
		Model(&models.ConnectionSession{}).
		Where("id = ?", sessionID).
		Update("last_heartbeat_at", now).Error; err != nil {
		return err
	}

	s.active.Heartbeat(sessionID)
	return nil
}

// CloseSession finalises the session, unregisters the active record, flushes chat, and emits audit/metrics.
func (s *SessionLifecycleService) CloseSession(ctx context.Context, params CloseSessionParams) error {
	if s == nil {
		return errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return errors.New("session lifecycle service: session id is required")
	}

	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = SessionStatusClosed
	}
	endedAt := params.EndedAt
	if endedAt.IsZero() {
		endedAt = s.timeNow()
	}

	var session models.ConnectionSession
	var alreadyClosed bool

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			return err
		}
		if session.ClosedAt != nil {
			alreadyClosed = true
			return nil
		}

		session.Status = status
		session.ClosedAt = &endedAt
		session.LastHeartbeatAt = endedAt
		if err := tx.Save(&session).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.ConnectionSessionParticipant{}).
			Where("session_id = ? AND left_at IS NULL", sessionID).
			Update("left_at", endedAt).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if alreadyClosed {
		return nil
	}

	messages := s.active.ConsumeChatBuffer(sessionID)
	if len(messages) > 0 && s.chat != nil {
		if err := s.chat.PersistMessages(ctx, sessionID, messages); err != nil {
			return err
		}
	}

	s.active.UnregisterSession(sessionID)
	monitoring.AdjustActiveSessions(-1)

	if !session.StartedAt.IsZero() {
		monitoring.RecordSessionClosed(endedAt.Sub(session.StartedAt))
	}

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.closed",
			Resource: fmt.Sprintf("connection:%s", session.ConnectionID),
			Result:   resultFromStatus(status),
			Metadata: map[string]any{
				"session_id": session.ID,
				"status":     status,
				"reason":     params.Reason,
			},
		}, session.OwnerUserID)); err != nil {
			return err
		}
	}

	if s.recorder != nil {
		if err := s.recorder.OnSessionClosed(ctx, &session, params.Reason); err != nil {
			return err
		}
	}

	return nil
}

// FailSession marks the session as failed and delegates to CloseSession.
func (s *SessionLifecycleService) FailSession(ctx context.Context, sessionID, reason string, actor SessionActor) error {
	return s.CloseSession(ctx, CloseSessionParams{
		SessionID: sessionID,
		Status:    SessionStatusFailed,
		Reason:    reason,
		Actor:     actor,
	})
}

// GetActiveSession returns a snapshot of the active session record, if present.
func (s *SessionLifecycleService) GetActiveSession(sessionID string) (*ActiveSessionRecord, bool) {
	if s == nil || s.active == nil {
		return nil, false
	}
	return s.active.GetSession(sessionID)
}

// AuthorizeSessionAccess ensures the supplied user can interact with the session.
func (s *SessionLifecycleService) AuthorizeSessionAccess(ctx context.Context, sessionID, userID string) (*models.ConnectionSession, error) {
	if s == nil {
		return nil, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)
	sessionID = strings.TrimSpace(sessionID)
	userID = strings.TrimSpace(userID)
	if sessionID == "" {
		return nil, errors.New("session lifecycle service: session id is required")
	}
	if userID == "" {
		return nil, errors.New("session lifecycle service: user id is required")
	}

	var session models.ConnectionSession
	if err := s.db.WithContext(ctx).
		First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if strings.EqualFold(session.OwnerUserID, userID) {
		return &session, nil
	}

	var participantCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.ConnectionSessionParticipant{}).
		Where("session_id = ? AND user_id = ? AND (left_at IS NULL)", sessionID, userID).
		Count(&participantCount).Error; err != nil {
		return nil, err
	}
	if participantCount > 0 {
		return &session, nil
	}

	if active, ok := s.active.GetSession(sessionID); ok {
		if strings.EqualFold(active.OwnerUserID, userID) {
			return &session, nil
		}
		if active.Participants != nil {
			if p, exists := active.Participants[userID]; exists && p != nil {
				return &session, nil
			}
		}
	}

	return nil, ErrSessionAccessDenied
}

// AddParticipant registers a participant both in-memory and in persistence.
func (s *SessionLifecycleService) AddParticipant(ctx context.Context, params AddParticipantParams) (ActiveSessionParticipant, error) {
	if s == nil {
		return ActiveSessionParticipant{}, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)

	if strings.TrimSpace(params.SessionID) == "" {
		return ActiveSessionParticipant{}, errors.New("session lifecycle service: session id is required")
	}
	if strings.TrimSpace(params.UserID) == "" {
		return ActiveSessionParticipant{}, errors.New("session lifecycle service: user id is required")
	}

	joinedAt := params.JoinedAt
	if joinedAt.IsZero() {
		joinedAt = s.timeNow()
	}

	participant, err := s.active.AddParticipant(params.SessionID, ActiveSessionParticipant{
		SessionID:  params.SessionID,
		UserID:     params.UserID,
		UserName:   params.UserName,
		Role:       defaultRole(params.Role),
		AccessMode: defaultAccessMode(params.AccessMode),
		JoinedAt:   joinedAt,
	})
	if err != nil {
		return ActiveSessionParticipant{}, err
	}

	model := models.ConnectionSessionParticipant{
		SessionID:            params.SessionID,
		UserID:               params.UserID,
		Role:                 participant.Role,
		AccessMode:           participant.AccessMode,
		GrantedByUserID:      params.GrantedByUserID,
		JoinedAt:             participant.JoinedAt,
		LeftAt:               nil,
		ConsentedToRecording: params.ConsentedToRecording,
	}

	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"role":                   model.Role,
			"access_mode":            model.AccessMode,
			"granted_by_user_id":     model.GrantedByUserID,
			"joined_at":              model.JoinedAt,
			"left_at":                gorm.Expr("NULL"),
			"consented_to_recording": model.ConsentedToRecording,
		}),
	}).Create(&model).Error
	if err != nil {
		s.active.RemoveParticipant(params.SessionID, params.UserID)
		return ActiveSessionParticipant{}, err
	}

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.participant_joined",
			Resource: fmt.Sprintf("session:%s", params.SessionID),
			Result:   "success",
			Metadata: map[string]any{
				"user_id": params.UserID,
				"role":    participant.Role,
			},
		}, params.Actor.UserID)); err != nil {
			return ActiveSessionParticipant{}, err
		}
	}

	monitoring.RecordSessionShareEvent("participant_joined")

	return participant, nil
}

// RemoveParticipant ejects the participant and marks the database record.
func (s *SessionLifecycleService) RemoveParticipant(ctx context.Context, params RemoveParticipantParams) (bool, error) {
	if s == nil {
		return false, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)

	if strings.TrimSpace(params.SessionID) == "" || strings.TrimSpace(params.UserID) == "" {
		return false, errors.New("session lifecycle service: session id and user id are required")
	}

	if !s.active.RemoveParticipant(params.SessionID, params.UserID) {
		return false, nil
	}

	leftAt := params.LeftAt
	if leftAt.IsZero() {
		leftAt = s.timeNow()
	}

	if err := s.db.WithContext(ctx).
		Model(&models.ConnectionSessionParticipant{}).
		Where("session_id = ? AND user_id = ?", params.SessionID, params.UserID).
		Updates(map[string]any{
			"left_at":     leftAt,
			"access_mode": "read",
		}).Error; err != nil {
		return false, err
	}

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.participant_left",
			Resource: fmt.Sprintf("session:%s", params.SessionID),
			Result:   "success",
			Metadata: map[string]any{
				"user_id": params.UserID,
			},
		}, params.Actor.UserID)); err != nil {
			return false, err
		}
	}

	monitoring.RecordSessionShareEvent("participant_left")

	return true, nil
}

// GrantWriteAccess designates the supplied participant as the sole writer.
func (s *SessionLifecycleService) GrantWriteAccess(ctx context.Context, params GrantWriteParams) (ActiveSessionParticipant, error) {
	if s == nil {
		return ActiveSessionParticipant{}, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)

	if strings.TrimSpace(params.SessionID) == "" || strings.TrimSpace(params.UserID) == "" {
		return ActiveSessionParticipant{}, errors.New("session lifecycle service: session id and user id are required")
	}

	participant, err := s.active.GrantWriteAccess(params.SessionID, params.UserID)
	if err != nil {
		return ActiveSessionParticipant{}, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		assignments := map[string]any{
			"access_mode":        "write",
			"granted_by_user_id": params.GrantedByUserID,
		}
		if err := tx.
			Model(&models.ConnectionSessionParticipant{}).
			Where("session_id = ? AND user_id = ?", params.SessionID, params.UserID).
			Updates(assignments).Error; err != nil {
			return err
		}

		return tx.
			Model(&models.ConnectionSessionParticipant{}).
			Where("session_id = ? AND user_id <> ?", params.SessionID, params.UserID).
			Update("access_mode", "read").Error
	})
	if err != nil {
		return ActiveSessionParticipant{}, err
	}

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.write_granted",
			Resource: fmt.Sprintf("session:%s", params.SessionID),
			Result:   "success",
			Metadata: map[string]any{
				"user_id": params.UserID,
			},
		}, params.Actor.UserID)); err != nil {
			return ActiveSessionParticipant{}, err
		}
	}

	monitoring.RecordSessionShareEvent("write_granted")

	return participant, nil
}

// RelinquishWriteAccess revokes write control from the participant and optionally grants it to another.
func (s *SessionLifecycleService) RelinquishWriteAccess(ctx context.Context, params RelinquishWriteParams) (ActiveSessionParticipant, *ActiveSessionParticipant, error) {
	if s == nil {
		return ActiveSessionParticipant{}, nil, errors.New("session lifecycle service: service not initialised")
	}
	ctx = ensureContext(ctx)

	if strings.TrimSpace(params.SessionID) == "" || strings.TrimSpace(params.UserID) == "" {
		return ActiveSessionParticipant{}, nil, errors.New("session lifecycle service: session id and user id are required")
	}

	participant, newWriter, err := s.active.RelinquishWriteAccess(params.SessionID, params.UserID)
	if err != nil {
		return ActiveSessionParticipant{}, nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		clearUpdates := map[string]any{
			"access_mode":        "read",
			"granted_by_user_id": gorm.Expr("NULL"),
		}
		if err := tx.
			Model(&models.ConnectionSessionParticipant{}).
			Where("session_id = ? AND user_id = ?", params.SessionID, params.UserID).
			Updates(clearUpdates).Error; err != nil {
			return err
		}

		if newWriter != nil {
			assignments := map[string]any{
				"access_mode": "write",
			}
			actorID := strings.TrimSpace(params.Actor.UserID)
			if actorID != "" {
				id := actorID
				assignments["granted_by_user_id"] = &id
			} else {
				assignments["granted_by_user_id"] = gorm.Expr("NULL")
			}
			if err := tx.
				Model(&models.ConnectionSessionParticipant{}).
				Where("session_id = ? AND user_id = ?", params.SessionID, newWriter.UserID).
				Updates(assignments).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return ActiveSessionParticipant{}, nil, err
	}

	if s.audit != nil {
		if err := s.audit.Log(ctx, buildAuditEntry(params.Actor, AuditEntry{
			Action:   "session.write_relinquished",
			Resource: fmt.Sprintf("session:%s", params.SessionID),
			Result:   "success",
			Metadata: map[string]any{
				"user_id": params.UserID,
				"new_write_uid": func() string {
					if newWriter != nil {
						return newWriter.UserID
					}
					return ""
				}(),
			},
		}, params.Actor.UserID)); err != nil {
			return ActiveSessionParticipant{}, nil, err
		}
	}

	monitoring.RecordSessionShareEvent("write_relinquished")

	return participant, newWriter, nil
}

func encodeMetadata(meta map[string]any) (datatypes.JSON, error) {
	if len(meta) == 0 {
		return datatypes.JSON(json.RawMessage(`{}`)), nil
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("session lifecycle service: marshal metadata: %w", err)
	}
	return datatypes.JSON(payload), nil
}

func cloneMetadata(source map[string]any) map[string]any {
	if len(source) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func defaultRole(role string) string {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return "participant"
	}
	return role
}

func defaultAccessMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode != "write" {
		return "read"
	}
	return mode
}

func buildAuditEntry(actor SessionActor, entry AuditEntry, fallbackUserID string) AuditEntry {
	username := strings.TrimSpace(actor.Username)
	if username == "" {
		username = entry.Username
	}
	entry.Username = username
	entry.IPAddress = strings.TrimSpace(actor.IPAddress)
	entry.UserAgent = strings.TrimSpace(actor.UserAgent)

	userID := strings.TrimSpace(actor.UserID)
	if userID == "" {
		userID = strings.TrimSpace(fallbackUserID)
	}
	if userID != "" {
		entry.UserID = &userID
	}
	return entry
}

func resultFromStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SessionStatusFailed:
		return "error"
	default:
		return "success"
	}
}
