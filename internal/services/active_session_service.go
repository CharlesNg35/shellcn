package services

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/realtime"
)

// ErrActiveSessionExists indicates a user already has a live session for the connection.
var ErrActiveSessionExists = errors.New("active session already exists for user and connection")

// ActiveSessionRecord represents an in-memory record of an active connection session.
type ActiveSessionRecord struct {
	ID             string         `json:"id"`
	ConnectionID   string         `json:"connection_id"`
	ConnectionName string         `json:"connection_name,omitempty"`
	UserID         string         `json:"user_id"`
	UserName       string         `json:"user_name,omitempty"`
	TeamID         *string        `json:"team_id,omitempty"`
	ProtocolID     string         `json:"protocol_id"`
	StartedAt      time.Time      `json:"started_at"`
	LastSeenAt     time.Time      `json:"last_seen_at"`
	Host           string         `json:"host,omitempty"`
	Port           int            `json:"port,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ListActiveOptions controls how active sessions are filtered.
type ListActiveOptions struct {
	UserID       string
	TeamIDs      []string
	IncludeAll   bool
	IncludeTeams bool
}

// ActiveSessionService stores active sessions in memory and emits realtime events.
type ActiveSessionService struct {
	mu            sync.RWMutex
	sessions      map[string]*ActiveSessionRecord
	userConnIndex map[string]string
	hub           *realtime.Hub
	timeNow       func() time.Time
}

// NewActiveSessionService constructs an ActiveSessionService backed by the supplied realtime hub.
func NewActiveSessionService(hub *realtime.Hub) *ActiveSessionService {
	return &ActiveSessionService{
		sessions:      make(map[string]*ActiveSessionRecord),
		userConnIndex: make(map[string]string),
		hub:           hub,
		timeNow:       time.Now,
	}
}

// RegisterSession registers a new active session. An error is returned if the user already has
// an active session for the connection.
func (s *ActiveSessionService) RegisterSession(session *ActiveSessionRecord) error {
	if session == nil {
		return errors.New("active session: record is required")
	}
	if session.ID == "" {
		return errors.New("active session: id is required")
	}
	if session.ConnectionID == "" {
		return errors.New("active session: connection id is required")
	}
	if session.UserID == "" {
		return errors.New("active session: user id is required")
	}
	if session.ProtocolID == "" {
		return errors.New("active session: protocol id is required")
	}

	now := s.timeNow()
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	if session.LastSeenAt.IsZero() {
		session.LastSeenAt = session.StartedAt
	}

	record := cloneSessionRecord(session)

	indexKey := userConnectionKey(record.UserID, record.ConnectionID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[record.ID]; exists {
		return fmt.Errorf("active session: session %s already registered", record.ID)
	}
	if existingID, exists := s.userConnIndex[indexKey]; exists {
		return fmt.Errorf("%w: existing session id %s", ErrActiveSessionExists, existingID)
	}

	s.sessions[record.ID] = record
	s.userConnIndex[indexKey] = record.ID

	s.broadcastEvent("session.opened", record)

	return nil
}

// UnregisterSession removes the session from the registry and notifies listeners.
func (s *ActiveSessionService) UnregisterSession(sessionID string) {
	if sessionID == "" {
		return
	}

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if exists {
		delete(s.sessions, sessionID)
		delete(s.userConnIndex, userConnectionKey(record.UserID, record.ConnectionID))
	}
	s.mu.Unlock()

	if !exists {
		return
	}

	s.broadcastEvent("session.closed", map[string]any{
		"id":            record.ID,
		"connection_id": record.ConnectionID,
		"user_id":       record.UserID,
	})
}

// Heartbeat updates the last seen timestamp for the session.
func (s *ActiveSessionService) Heartbeat(sessionID string) {
	if sessionID == "" {
		return
	}

	now := s.timeNow()

	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.LastSeenAt = now
	}
}

// ListActive returns copies of the sessions visible to the requesting user.
func (s *ActiveSessionService) ListActive(opts ListActiveOptions) []ActiveSessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	includeTeams := opts.IncludeTeams && len(opts.TeamIDs) > 0
	teamLookup := make(map[string]struct{})
	if includeTeams {
		for _, id := range opts.TeamIDs {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				teamLookup[trimmed] = struct{}{}
			}
		}
		includeTeams = len(teamLookup) > 0
	}

	results := make([]ActiveSessionRecord, 0, len(s.sessions))
	for _, record := range s.sessions {
		if !opts.IncludeAll {
			switch {
			case opts.UserID != "" && record.UserID == opts.UserID:
				// ok
			case includeTeams && record.TeamID != nil:
				if _, ok := teamLookup[strings.TrimSpace(*record.TeamID)]; ok {
					// ok
				} else {
					continue
				}
			default:
				continue
			}
		}
		results = append(results, *cloneSessionRecord(record))
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].LastSeenAt.After(results[j].LastSeenAt)
	})

	return results
}

// HasActiveSession checks whether the user already has a session for the connection.
func (s *ActiveSessionService) HasActiveSession(userID, connectionID string) bool {
	indexKey := userConnectionKey(strings.TrimSpace(userID), strings.TrimSpace(connectionID))
	if indexKey == ":" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.userConnIndex[indexKey]
	return exists
}

// CleanupStale removes sessions whose LastSeenAt is older than the grace period.
func (s *ActiveSessionService) CleanupStale(gracePeriod time.Duration) {
	if gracePeriod <= 0 {
		return
	}
	threshold := s.timeNow().Add(-gracePeriod)

	s.mu.Lock()
	expired := make([]*ActiveSessionRecord, 0)
	for id, record := range s.sessions {
		if record.LastSeenAt.Before(threshold) {
			delete(s.sessions, id)
			delete(s.userConnIndex, userConnectionKey(record.UserID, record.ConnectionID))
			expired = append(expired, cloneSessionRecord(record))
		}
	}
	s.mu.Unlock()

	for _, record := range expired {
		s.broadcastEvent("session.closed", map[string]any{
			"id":            record.ID,
			"connection_id": record.ConnectionID,
			"user_id":       record.UserID,
			"reason":        "timeout",
		})
	}
}

// Count returns the number of active sessions.
func (s *ActiveSessionService) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *ActiveSessionService) broadcastEvent(event string, data any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastStream(realtime.StreamConnectionSessions, realtime.Message{
		Event: event,
		Data:  data,
	})
}

func cloneSessionRecord(session *ActiveSessionRecord) *ActiveSessionRecord {
	if session == nil {
		return nil
	}
	clone := *session
	if session.TeamID != nil {
		team := *session.TeamID
		clone.TeamID = &team
	}
	if session.Metadata != nil {
		meta := make(map[string]any, len(session.Metadata))
		for key, value := range session.Metadata {
			meta[key] = value
		}
		clone.Metadata = meta
	}
	return &clone
}

func userConnectionKey(userID, connectionID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(userID), strings.TrimSpace(connectionID))
}
