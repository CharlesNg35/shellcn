package services

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/multierr"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
)

const (
	// RecordingModeDisabled prevents all recordings.
	RecordingModeDisabled = "disabled"
	// RecordingModeOptional records when explicitly enabled.
	RecordingModeOptional = "optional"
	// RecordingModeForced records every compatible session.
	RecordingModeForced = "forced"

	defaultRecordingStorage = "filesystem"
	defaultTerminalWidth    = 80
	defaultTerminalHeight   = 24
)

// RecorderPolicy captures runtime configuration for session recordings.
type RecorderPolicy struct {
	Mode           string
	Storage        string
	RetentionDays  int
	RequireConsent bool
}

// RecordingStatus summarises the recording state for a session.
type RecordingStatus struct {
	SessionID      string     `json:"session_id"`
	Active         bool       `json:"active"`
	StartedAt      time.Time  `json:"started_at"`
	LastEventAt    time.Time  `json:"last_event_at"`
	BytesRecorded  int64      `json:"bytes_recorded"`
	StoragePath    string     `json:"storage_path,omitempty"`
	RecordID       string     `json:"record_id,omitempty"`
	Duration       int64      `json:"duration_seconds,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Checksum       string     `json:"checksum,omitempty"`
	StorageKind    string     `json:"storage_kind,omitempty"`
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
	PolicyMode     string     `json:"-"`
}

// RecordingScope represents the visibility scope requested when listing recordings.
type RecordingScope string

const (
	// RecordingScopePersonal limits results to the caller's own sessions.
	RecordingScopePersonal RecordingScope = "personal"
	// RecordingScopeTeam limits results to the caller's teams.
	RecordingScopeTeam RecordingScope = "team"
	// RecordingScopeAll returns all recordings irrespective of ownership.
	RecordingScopeAll RecordingScope = "all"
)

// ListRecordingsOptions controls how session recordings are queried.
type ListRecordingsOptions struct {
	Scope           RecordingScope
	UserID          string
	TeamIDs         []string
	TeamID          string
	ProtocolID      string
	ConnectionID    string
	SessionID       string
	OwnerUserID     string
	CreatedByUserID string
	Limit           int
	Offset          int
	Sort            string
}

// RecordingSummary summarises stored recording metadata for administrative listings.
type RecordingSummary struct {
	RecordID          string     `json:"record_id"`
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

// RecorderOption customises RecorderService construction.
type RecorderOption func(*RecorderService)

// WithRecorderPolicy overrides the policy used by the recorder service.
func WithRecorderPolicy(policy RecorderPolicy) RecorderOption {
	return func(s *RecorderService) {
		s.policy = policy
	}
}

// WithRecorderClock injects a custom clock (primarily for tests).
func WithRecorderClock(clock func() time.Time) RecorderOption {
	return func(s *RecorderService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// RecorderService coordinates active session recordings and persistence.
type RecorderService struct {
	db       *gorm.DB
	store    RecorderStore
	policy   RecorderPolicy
	policyMu sync.RWMutex

	now func() time.Time

	mu     sync.RWMutex
	active map[string]*recordingContext
}

func (s *RecorderService) updatePolicy(policy RecorderPolicy) {
	normalised := normalisePolicy(policy)
	s.policyMu.Lock()
	s.policy = normalised
	s.policyMu.Unlock()
}

func (s *RecorderService) currentPolicy() RecorderPolicy {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	return s.policy
}

func normalisePolicy(policy RecorderPolicy) RecorderPolicy {
	mode := strings.ToLower(strings.TrimSpace(policy.Mode))
	switch mode {
	case RecordingModeDisabled, RecordingModeForced, RecordingModeOptional:
	default:
		mode = RecordingModeOptional
	}

	storage := strings.ToLower(strings.TrimSpace(policy.Storage))
	if storage == "" {
		storage = defaultRecordingStorage
	}

	retention := policy.RetentionDays
	if retention < 0 {
		retention = 0
	}

	return RecorderPolicy{
		Mode:           mode,
		Storage:        storage,
		RetentionDays:  retention,
		RequireConsent: policy.RequireConsent,
	}
}

// NewRecorderService constructs a RecorderService bound to the supplied storage backend.
func NewRecorderService(db *gorm.DB, store RecorderStore, opts ...RecorderOption) (*RecorderService, error) {
	if db == nil {
		return nil, errors.New("recorder service: db is required")
	}
	if store == nil {
		return nil, errors.New("recorder service: store is required")
	}

	s := &RecorderService{
		db:     db,
		store:  store,
		now:    time.Now,
		active: make(map[string]*recordingContext),
	}
	s.updatePolicy(normalisePolicy(RecorderPolicy{
		Mode:           RecordingModeOptional,
		Storage:        defaultRecordingStorage,
		RetentionDays:  90,
		RequireConsent: true,
	}))
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	return s, nil
}

// LoadRecorderPolicy fetches recorder configuration from system settings, falling back to defaults.
func LoadRecorderPolicy(ctx context.Context, db *gorm.DB) RecorderPolicy {
	policy := RecorderPolicy{
		Mode:           RecordingModeOptional,
		Storage:        defaultRecordingStorage,
		RetentionDays:  90,
		RequireConsent: true,
	}
	if db == nil {
		return policy
	}
	mode, err := database.GetSystemSetting(ctx, db, "recording.mode")
	if err == nil && strings.TrimSpace(mode) != "" {
		policy.Mode = strings.ToLower(strings.TrimSpace(mode))
	}
	storage, err := database.GetSystemSetting(ctx, db, "recording.storage")
	if err == nil && strings.TrimSpace(storage) != "" {
		policy.Storage = strings.ToLower(strings.TrimSpace(storage))
	}
	if days, err := database.GetSystemSetting(ctx, db, "recording.retention_days"); err == nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(days)); parseErr == nil && parsed >= 0 {
			policy.RetentionDays = parsed
		}
	}
	requireConsent, err := database.GetSystemSetting(ctx, db, "recording.require_consent")
	if err == nil && strings.TrimSpace(requireConsent) != "" {
		if parsed, parseErr := strconv.ParseBool(strings.TrimSpace(requireConsent)); parseErr == nil {
			policy.RequireConsent = parsed
		}
	}
	return normalisePolicy(policy)
}

// UpdatePolicy replaces the active recorder policy at runtime and returns the applied configuration.
func (s *RecorderService) UpdatePolicy(policy RecorderPolicy) RecorderPolicy {
	if s == nil {
		return normalisePolicy(policy)
	}
	s.updatePolicy(policy)
	return s.currentPolicy()
}

// Policy returns a snapshot of the currently applied recorder policy.
func (s *RecorderService) Policy() RecorderPolicy {
	if s == nil {
		return normalisePolicy(RecorderPolicy{
			Mode:           RecordingModeOptional,
			Storage:        defaultRecordingStorage,
			RetentionDays:  90,
			RequireConsent: true,
		})
	}
	return s.currentPolicy()
}

// OnSessionStarted initialises a recording pipeline if the policy allows capturing the session.
func (s *RecorderService) OnSessionStarted(ctx context.Context, session *models.ConnectionSession) error {
	if s == nil {
		return nil
	}
	if session == nil {
		return errors.New("recorder service: session is required")
	}
	if !s.shouldRecord(session) {
		return nil
	}

	resource := RecordingResource{
		SessionID:  session.ID,
		ProtocolID: session.ProtocolID,
		StartedAt:  session.StartedAt,
	}

	writer, err := s.store.Create(ctx, resource)
	if err != nil {
		return fmt.Errorf("recorder service: allocate store writer: %w", err)
	}

	meta := decodeSessionMetadata(session.Metadata)
	width, height, term := extractTerminalMetadata(meta)

	context, err := newRecordingContext(writer, session.ID, session.ProtocolID, session.StartedAt, width, height, term)
	if err != nil {
		_ = writer.Writer.Close()
		return err
	}

	s.mu.Lock()
	s.active[session.ID] = context
	s.mu.Unlock()

	monitoring.RecordSessionRecordingEvent("started")
	return nil
}

// OnSessionClosed finalises the recording (if active) and persists metadata.
func (s *RecorderService) OnSessionClosed(ctx context.Context, session *models.ConnectionSession, reason string) error {
	if s == nil || session == nil {
		return nil
	}

	recCtx := s.detach(session.ID)
	if recCtx == nil {
		return nil
	}

	record, err := s.finalizeRecording(ctx, recCtx, session, session.OwnerUserID, reason, s.now())
	if err != nil {
		return err
	}
	if record != nil {
		monitoring.RecordSessionRecordingEvent("completed")
	}
	return nil
}

// RecordStream appends terminal output to the active recording buffer.
func (s *RecorderService) RecordStream(sessionID, stream string, payload []byte) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(payload) == 0 {
		return
	}

	s.mu.RLock()
	recCtx := s.active[sessionID]
	s.mu.RUnlock()
	if recCtx == nil {
		return
	}

	event := recordingEvent{
		at:      s.now(),
		stream:  strings.ToLower(strings.TrimSpace(stream)),
		payload: append([]byte(nil), payload...),
	}
	recCtx.enqueue(event)
}

// StopRecording finalises an active recording without closing the session.
func (s *RecorderService) StopRecording(ctx context.Context, sessionID, actorUserID, reason string) (*models.ConnectionSessionRecord, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("recorder service: session id is required")
	}

	recCtx := s.detach(sessionID)
	if recCtx == nil {
		return nil, nil
	}

	var session models.ConnectionSession
	if err := s.db.WithContext(ctx).
		First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	record, err := s.finalizeRecording(ctx, recCtx, &session, actorUserID, reason, s.now())
	if err != nil {
		return nil, err
	}
	if record != nil {
		monitoring.RecordSessionRecordingEvent("stopped")
	}
	return record, nil
}

// Status reports the current recording status for the supplied session.
func (s *RecorderService) Status(ctx context.Context, sessionID string) (RecordingStatus, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return RecordingStatus{}, errors.New("recorder service: session id is required")
	}

	s.mu.RLock()
	recCtx := s.active[sessionID]
	s.mu.RUnlock()
	if recCtx != nil {
		status := recCtx.status()
		status.PolicyMode = s.currentPolicy().Mode
		return status, nil
	}

	var record models.ConnectionSessionRecord
	err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RecordingStatus{SessionID: sessionID, Active: false}, nil
		}
		return RecordingStatus{}, err
	}

	status := RecordingStatus{
		SessionID:      sessionID,
		Active:         false,
		BytesRecorded:  record.SizeBytes,
		StoragePath:    record.StoragePath,
		RecordID:       record.ID,
		StorageKind:    record.StorageKind,
		Checksum:       record.Checksum,
		Duration:       record.DurationSeconds,
		RetentionUntil: record.RetentionUntil,
	}
	status.PolicyMode = s.currentPolicy().Mode

	var session models.ConnectionSession
	if err := s.db.WithContext(ctx).
		Select("started_at", "closed_at").
		First(&session, "id = ?", sessionID).Error; err == nil {
		if !session.StartedAt.IsZero() {
			status.StartedAt = session.StartedAt
		}
		if session.ClosedAt != nil && !session.ClosedAt.IsZero() {
			status.CompletedAt = session.ClosedAt
		}
	}
	if status.StartedAt.IsZero() {
		status.StartedAt = record.CreatedAt
	}
	status.LastEventAt = record.CreatedAt
	if status.CompletedAt == nil {
		completed := record.CreatedAt
		status.CompletedAt = &completed
	}
	return status, nil
}

// ListRecordings returns paginated recording summaries respecting the supplied scope.
func (s *RecorderService) ListRecordings(ctx context.Context, opts ListRecordingsOptions) ([]RecordingSummary, int64, error) {
	if s == nil {
		return nil, 0, errors.New("recorder service: service not initialised")
	}
	ctx = ensureContext(ctx)

	scope := opts.Scope
	if scope == "" {
		scope = RecordingScopePersonal
	}

	userID := strings.TrimSpace(opts.UserID)
	if scope != RecordingScopeAll && userID == "" {
		return nil, 0, errors.New("recorder service: user id is required for scoped queries")
	}

	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	teamFilter := strings.TrimSpace(opts.TeamID)
	sanitizeTeamIDs := func(ids []string) []string {
		cleaned := make([]string, 0, len(ids))
		for _, id := range ids {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		return cleaned
	}

	effectiveScope := scope

	switch scope {
	case RecordingScopeTeam:
		if strings.EqualFold(teamFilter, "personal") {
			effectiveScope = RecordingScopePersonal
			opts.TeamID = ""
			opts.TeamIDs = nil
		} else {
			allowedTeams := sanitizeTeamIDs(opts.TeamIDs)
			if teamFilter != "" {
				allowedTeams = sanitizeTeamIDs([]string{teamFilter})
				opts.TeamID = strings.TrimSpace(teamFilter)
			} else {
				opts.TeamID = ""
			}
			if len(allowedTeams) == 0 {
				return []RecordingSummary{}, 0, nil
			}
			opts.TeamIDs = allowedTeams
		}
	case RecordingScopeAll:
		if strings.EqualFold(teamFilter, "personal") {
			effectiveScope = RecordingScopePersonal
			opts.TeamID = ""
			opts.TeamIDs = nil
		} else if teamFilter != "" {
			opts.TeamID = teamFilter
		}
	default:
		opts.TeamID = ""
		opts.TeamIDs = nil
	}

	scope = effectiveScope
	opts.Scope = scope

	const selectClause = `
        connection_session_records.id AS record_id,
        connection_session_records.session_id AS session_id,
        connection_session_records.storage_kind AS storage_kind,
        connection_session_records.storage_path AS storage_path,
        connection_session_records.size_bytes AS size_bytes,
        connection_session_records.duration_seconds AS duration_seconds,
        connection_session_records.checksum AS checksum,
        connection_session_records.created_by_user_id AS created_by_user_id,
        connection_session_records.created_at AS created_at,
        connection_session_records.retention_until AS retention_until,
        connection_sessions.connection_id AS connection_id,
        connection_sessions.protocol_id AS protocol_id,
        connection_sessions.owner_user_id AS owner_user_id,
        connection_sessions.team_id AS team_id,
        connections.name AS connection_name,
        owner.username AS owner_user_name,
        creator.username AS created_by_user_name
    `

	applyFilters := func(q *gorm.DB) *gorm.DB {
		switch scope {
		case RecordingScopePersonal:
			q = q.Where("(connection_sessions.owner_user_id = ? OR connection_session_records.created_by_user_id = ?)", userID, userID)
		case RecordingScopeTeam:
			teamIDs := sanitizeTeamIDs(opts.TeamIDs)
			if opts.TeamID != "" && len(teamIDs) > 0 {
				q = q.Where("connection_sessions.team_id IN ?", teamIDs)
			} else if len(teamIDs) > 0 {
				q = q.Where("(connection_sessions.team_id IN ? OR connection_sessions.owner_user_id = ? OR connection_session_records.created_by_user_id = ?)", teamIDs, userID, userID)
			} else {
				q = q.Where("(connection_sessions.owner_user_id = ? OR connection_session_records.created_by_user_id = ?)", userID, userID)
			}
		case RecordingScopeAll:
			if trimmed := strings.TrimSpace(opts.TeamID); trimmed != "" {
				q = q.Where("connection_sessions.team_id = ?", trimmed)
			}
		}

		if trimmed := strings.TrimSpace(opts.ProtocolID); trimmed != "" {
			q = q.Where("connection_sessions.protocol_id = ?", trimmed)
		}
		if trimmed := strings.TrimSpace(opts.ConnectionID); trimmed != "" {
			q = q.Where("connection_sessions.connection_id = ?", trimmed)
		}
		if trimmed := strings.TrimSpace(opts.SessionID); trimmed != "" {
			q = q.Where("connection_session_records.session_id = ?", trimmed)
		}
		if trimmed := strings.TrimSpace(opts.OwnerUserID); trimmed != "" {
			q = q.Where("connection_sessions.owner_user_id = ?", trimmed)
		}
		if trimmed := strings.TrimSpace(opts.CreatedByUserID); trimmed != "" {
			q = q.Where("connection_session_records.created_by_user_id = ?", trimmed)
		}
		return q
	}

	dataQuery := s.db.WithContext(ctx).
		Model(&models.ConnectionSessionRecord{}).
		Joins("JOIN connection_sessions ON connection_sessions.id = connection_session_records.session_id").
		Joins("LEFT JOIN connections ON connections.id = connection_sessions.connection_id").
		Joins("LEFT JOIN users owner ON owner.id = connection_sessions.owner_user_id").
		Joins("LEFT JOIN users creator ON creator.id = connection_session_records.created_by_user_id").
		Select(selectClause)
	dataQuery = applyFilters(dataQuery)

	countQuery := s.db.WithContext(ctx).
		Model(&models.ConnectionSessionRecord{}).
		Joins("JOIN connection_sessions ON connection_sessions.id = connection_session_records.session_id")
	countQuery = applyFilters(countQuery)

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("recorder service: count recordings: %w", err)
	}
	if total == 0 {
		return []RecordingSummary{}, 0, nil
	}

	orderExpr := "connection_session_records.created_at DESC"
	switch strings.ToLower(strings.TrimSpace(opts.Sort)) {
	case "oldest":
		orderExpr = "connection_session_records.created_at ASC"
	case "size_desc":
		orderExpr = "connection_session_records.size_bytes DESC, connection_session_records.created_at DESC"
	case "size_asc":
		orderExpr = "connection_session_records.size_bytes ASC, connection_session_records.created_at DESC"
	}

	dataQuery = dataQuery.Order(orderExpr).Limit(limit).Offset(offset)

	type recordingRow struct {
		RecordID          string     `gorm:"column:record_id"`
		SessionID         string     `gorm:"column:session_id"`
		StorageKind       string     `gorm:"column:storage_kind"`
		StoragePath       string     `gorm:"column:storage_path"`
		SizeBytes         int64      `gorm:"column:size_bytes"`
		DurationSeconds   int64      `gorm:"column:duration_seconds"`
		Checksum          string     `gorm:"column:checksum"`
		CreatedByUserID   string     `gorm:"column:created_by_user_id"`
		CreatedByUserName string     `gorm:"column:created_by_user_name"`
		CreatedAt         time.Time  `gorm:"column:created_at"`
		RetentionUntil    *time.Time `gorm:"column:retention_until"`
		ConnectionID      string     `gorm:"column:connection_id"`
		ConnectionName    string     `gorm:"column:connection_name"`
		ProtocolID        string     `gorm:"column:protocol_id"`
		OwnerUserID       string     `gorm:"column:owner_user_id"`
		OwnerUserName     string     `gorm:"column:owner_user_name"`
		TeamID            *string    `gorm:"column:team_id"`
	}

	rows := make([]recordingRow, 0, limit)
	if err := dataQuery.Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("recorder service: list recordings: %w", err)
	}

	summaries := make([]RecordingSummary, 0, len(rows))
	for _, row := range rows {
		summary := RecordingSummary{
			RecordID:          row.RecordID,
			SessionID:         row.SessionID,
			ConnectionID:      row.ConnectionID,
			ConnectionName:    row.ConnectionName,
			ProtocolID:        row.ProtocolID,
			OwnerUserID:       row.OwnerUserID,
			OwnerUserName:     row.OwnerUserName,
			TeamID:            row.TeamID,
			CreatedByUserID:   row.CreatedByUserID,
			CreatedByUserName: row.CreatedByUserName,
			StorageKind:       row.StorageKind,
			StoragePath:       row.StoragePath,
			SizeBytes:         row.SizeBytes,
			DurationSeconds:   row.DurationSeconds,
			Checksum:          row.Checksum,
			CreatedAt:         row.CreatedAt,
			RetentionUntil:    row.RetentionUntil,
		}
		summaries = append(summaries, summary)
	}

	return summaries, total, nil
}

// GetRecord fetches a recording and its parent session metadata without opening storage.
func (s *RecorderService) GetRecord(ctx context.Context, recordID string) (models.ConnectionSessionRecord, models.ConnectionSession, error) {
	if s == nil {
		return models.ConnectionSessionRecord{}, models.ConnectionSession{}, errors.New("recorder service: service not initialised")
	}
	ctx = ensureContext(ctx)
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return models.ConnectionSessionRecord{}, models.ConnectionSession{}, errors.New("recorder service: record id is required")
	}

	var record models.ConnectionSessionRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", recordID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ConnectionSessionRecord{}, models.ConnectionSession{}, ErrSessionNotFound
		}
		return models.ConnectionSessionRecord{}, models.ConnectionSession{}, err
	}

	var session models.ConnectionSession
	if err := s.db.WithContext(ctx).
		Select("id", "connection_id", "protocol_id", "owner_user_id", "team_id").
		First(&session, "id = ?", record.SessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return record, models.ConnectionSession{}, ErrSessionNotFound
		}
		return record, models.ConnectionSession{}, err
	}

	return record, session, nil
}

// OpenRecording returns a reader for the stored recording along with metadata.
func (s *RecorderService) OpenRecording(ctx context.Context, recordID string) (io.ReadCloser, models.ConnectionSessionRecord, error) {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return nil, models.ConnectionSessionRecord{}, errors.New("recorder service: record id is required")
	}

	var record models.ConnectionSessionRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", recordID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, record, ErrSessionNotFound
		}
		return nil, record, err
	}

	reader, err := s.store.Open(ctx, record.StoragePath)
	if err != nil {
		return nil, record, err
	}

	return reader, record, nil
}

// DeleteRecording removes a stored recording and its metadata.
func (s *RecorderService) DeleteRecording(ctx context.Context, recordID string) error {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return errors.New("recorder service: record id is required")
	}

	var record models.ConnectionSessionRecord
	if err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&record, "id = ?", recordID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	if err := s.store.Delete(ctx, record.StoragePath); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Delete(&record).Error
}

// CleanupExpired removes recordings whose retention window has elapsed. The optional limit constrains
// how many records are processed per invocation to avoid long-running jobs.
func (s *RecorderService) CleanupExpired(ctx context.Context, limit int) (int, error) {
	if s == nil {
		return 0, nil
	}
	ctx = ensureContext(ctx)
	if limit <= 0 {
		limit = 100
	}

	now := s.now()
	query := s.db.WithContext(ctx).
		Where("retention_until IS NOT NULL AND retention_until <= ?", now).
		Order("retention_until ASC").
		Limit(limit)

	var records []models.ConnectionSessionRecord
	if err := query.Find(&records).Error; err != nil {
		return 0, fmt.Errorf("recorder service: fetch expired recordings: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	var multiErr error
	purged := 0

	for _, record := range records {
		if record.StoragePath != "" {
			if err := s.store.Delete(ctx, record.StoragePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				multiErr = multierr.Append(multiErr, fmt.Errorf("recorder service: delete storage %s: %w", record.ID, err))
			}
		}
		if err := s.db.WithContext(ctx).Delete(&models.ConnectionSessionRecord{}, "id = ?", record.ID).Error; err != nil {
			multiErr = multierr.Append(multiErr, fmt.Errorf("recorder service: delete record %s: %w", record.ID, err))
			continue
		}
		purged++
		monitoring.RecordSessionRecordingEvent("purged")
	}

	return purged, multiErr
}

func (s *RecorderService) shouldRecord(session *models.ConnectionSession) bool {
	policy := s.currentPolicy()
	mode := strings.ToLower(strings.TrimSpace(policy.Mode))
	switch mode {
	case RecordingModeDisabled:
		return false
	case RecordingModeForced:
		return true
	default:
		// Optional recording requires explicit metadata opt-in.
		meta := decodeSessionMetadata(session.Metadata)
		if enabled, ok := lookupBool(meta, "recording_enabled"); ok {
			return enabled
		}
		return false
	}
}

func (s *RecorderService) detach(sessionID string) *recordingContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	recCtx := s.active[sessionID]
	if recCtx != nil {
		delete(s.active, sessionID)
		recCtx.stop()
	}
	return recCtx
}

func (s *RecorderService) finalizeRecording(ctx context.Context, recCtx *recordingContext, session *models.ConnectionSession, actorUserID, reason string, endedAt time.Time) (*models.ConnectionSessionRecord, error) {
	result, err := recCtx.finalize()
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	info, err := s.store.Stat(ctx, result.path)
	if err != nil {
		return nil, err
	}

	policy := s.currentPolicy()

	record := models.ConnectionSessionRecord{
		BaseModel: models.BaseModel{
			ID: result.id,
		},
		SessionID:       session.ID,
		StorageKind:     policy.Storage,
		StoragePath:     result.path,
		SizeBytes:       info.Size,
		DurationSeconds: int64(endedAt.Sub(session.StartedAt).Seconds()),
		Checksum:        result.checksum,
		CreatedByUserID: actorUserID,
		Metadata:        datatypes.JSON(result.metadata),
		Protected:       false,
	}
	if record.DurationSeconds < 0 {
		record.DurationSeconds = 0
	}
	if policy.RetentionDays > 0 {
		retain := session.StartedAt.AddDate(0, 0, policy.RetentionDays)
		record.RetentionUntil = &retain
	}

	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

type recordingEvent struct {
	at      time.Time
	stream  string
	payload []byte
}

type finalizeResult struct {
	id       string
	path     string
	checksum string
	metadata []byte
}

type recordingContext struct {
	sessionID  string
	protocolID string
	path       string
	startedAt  time.Time
	width      int
	height     int
	term       string

	writer io.WriteCloser
	gzip   *gzip.Writer
	buf    *bufio.Writer
	hash   hashWriter

	events chan recordingEvent
	done   chan struct{}
	err    atomic.Value

	stopOnce sync.Once

	writtenBytes atomic.Int64
	lastEvent    atomic.Pointer[time.Time]
}

type hashWriter interface {
	io.Writer
	Sum([]byte) []byte
}

func newRecordingContext(writer *RecordingWriter, sessionID, protocolID string, startedAt time.Time, width, height int, term string) (*recordingContext, error) {
	if writer == nil || writer.Writer == nil {
		return nil, errors.New("recorder context: writer is required")
	}
	if width <= 0 {
		width = defaultTerminalWidth
	}
	if height <= 0 {
		height = defaultTerminalHeight
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	hasher := sha256.New()
	multi := io.MultiWriter(writer.Writer, hasher)
	gzipWriter := gzip.NewWriter(multi)
	buffer := bufio.NewWriterSize(gzipWriter, 64*1024)

	ctx := &recordingContext{
		sessionID:  sessionID,
		protocolID: protocolID,
		path:       writer.Path,
		startedAt:  startedAt,
		width:      width,
		height:     height,
		term:       term,
		writer:     writer.Writer,
		gzip:       gzipWriter,
		buf:        buffer,
		hash:       hasher,
		events:     make(chan recordingEvent, 256),
		done:       make(chan struct{}),
	}

	if err := ctx.writeHeader(); err != nil {
		ctx.closeWriters()
		return nil, err
	}

	go ctx.loop()
	return ctx, nil
}

func (c *recordingContext) enqueue(event recordingEvent) {
	select {
	case c.events <- event:
	default:
		// Backpressure: block until event enqueued rather than dropping data.
		c.events <- event
	}
}

func (c *recordingContext) stop() {
	c.stopOnce.Do(func() {
		close(c.events)
	})
}

func (c *recordingContext) loop() {
	defer close(c.done)
	for event := range c.events {
		if err := c.writeEvent(event); err != nil {
			c.err.Store(err)
			break
		}
	}
	if err := c.flush(); err != nil {
		c.err.Store(err)
	}
}

func (c *recordingContext) writeHeader() error {
	header := map[string]any{
		"version":   2,
		"width":     c.width,
		"height":    c.height,
		"timestamp": c.startedAt.UTC().Unix(),
		"env": map[string]string{
			"TERM": c.term,
		},
	}
	line, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("recorder context: marshal header: %w", err)
	}
	if _, err := c.buf.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("recorder context: write header: %w", err)
	}
	c.writtenBytes.Add(int64(len(line) + 1))
	return nil
}

func (c *recordingContext) writeEvent(event recordingEvent) error {
	elapsed := event.at.Sub(c.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	streamCode := "o"
	if event.stream == "stdin" {
		streamCode = "i"
	}

	entry := []any{elapsed.Seconds(), streamCode, string(event.payload)}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("recorder context: marshal event: %w", err)
	}
	if _, err := c.buf.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("recorder context: write event: %w", err)
	}
	c.writtenBytes.Add(int64(len(line) + 1))
	c.lastEvent.Store(&event.at)
	return nil
}

func (c *recordingContext) flush() error {
	if err := c.buf.Flush(); err != nil {
		return fmt.Errorf("recorder context: flush buffer: %w", err)
	}
	if err := c.gzip.Close(); err != nil {
		return fmt.Errorf("recorder context: close gzip: %w", err)
	}
	return c.writer.Close()
}

func (c *recordingContext) closeWriters() {
	_ = c.buf.Flush()
	_ = c.gzip.Close()
	_ = c.writer.Close()
}

func (c *recordingContext) finalize() (*finalizeResult, error) {
	c.stop()
	<-c.done

	if err, _ := c.err.Load().(error); err != nil {
		return nil, err
	}

	hashBytes := c.hash.Sum(nil)
	checksum := hex.EncodeToString(hashBytes)

	meta := map[string]any{
		"protocol_id":  c.protocolID,
		"generated_at": time.Now().UTC(),
	}
	metaJSON, _ := json.Marshal(meta)

	return &finalizeResult{
		id:       uuidNewString(),
		path:     c.path,
		checksum: checksum,
		metadata: metaJSON,
	}, nil
}

func (c *recordingContext) status() RecordingStatus {
	status := RecordingStatus{
		SessionID:     c.sessionID,
		Active:        true,
		StartedAt:     c.startedAt,
		BytesRecorded: c.writtenBytes.Load(),
		StoragePath:   c.path,
	}
	if last := c.lastEvent.Load(); last != nil {
		status.LastEventAt = *last
	}
	return status
}

func decodeSessionMetadata(meta datatypes.JSON) map[string]any {
	if len(meta) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(meta, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func extractTerminalMetadata(meta map[string]any) (int, int, string) {
	width := lookupInt(meta, "terminal_width", defaultTerminalWidth)
	height := lookupInt(meta, "terminal_height", defaultTerminalHeight)
	term := lookupString(meta, "terminal_type", "xterm-256color")
	return width, height, term
}

func lookupBool(meta map[string]any, key string) (bool, bool) {
	raw, ok := meta[key]
	if !ok {
		return false, false
	}
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	}
	return false, false
}

func lookupInt(meta map[string]any, key string, fallback int) int {
	raw, ok := meta[key]
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func lookupString(meta map[string]any, key, fallback string) string {
	raw, ok := meta[key]
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func uuidNewString() string {
	return uuid.NewString()
}
