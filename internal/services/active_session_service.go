package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/realtime"
)

var (
	// ErrActiveSessionExists indicates a user already has a live session for the connection.
	ErrActiveSessionExists = errors.New("active session already exists for user and connection")
	// ErrConcurrentLimitReached indicates the connection's concurrent session limit has been reached.
	ErrConcurrentLimitReached = errors.New("active session concurrent limit reached")
)

const (
	// ConcurrentLimitReasonReached indicates the global concurrent limit for the connection was reached.
	ConcurrentLimitReasonReached = "limit_reached"

	defaultChatBufferSize = 100
)

// ConcurrentLimitError describes a concurrency enforcement failure with structured metadata.
type ConcurrentLimitError struct {
	ConnectionID string
	Limit        int
	Reason       string
}

// Error implements the error interface.
func (e *ConcurrentLimitError) Error() string {
	if e == nil {
		return ErrConcurrentLimitReached.Error()
	}
	reason := e.Reason
	if strings.TrimSpace(reason) == "" {
		reason = ConcurrentLimitReasonReached
	}
	return fmt.Sprintf("%s: connection=%s limit=%d reason=%s", ErrConcurrentLimitReached.Error(), e.ConnectionID, e.Limit, reason)
}

// Unwrap enables errors.Is/As checks against ErrConcurrentLimitReached.
func (e *ConcurrentLimitError) Unwrap() error {
	return ErrConcurrentLimitReached
}

// ActiveSessionParticipant describes an individual connected to an active session.
type ActiveSessionParticipant struct {
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name,omitempty"`
	Role       string    `json:"role"`
	AccessMode string    `json:"access_mode"`
	JoinedAt   time.Time `json:"joined_at"`
}

// ActiveSessionChatMessage is an ephemeral chat entry pending persistence.
type ActiveSessionChatMessage struct {
	MessageID string    `json:"message_id"`
	SessionID string    `json:"session_id"`
	AuthorID  string    `json:"author_id"`
	Author    string    `json:"author,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ActiveSessionRecord represents an in-memory record of an active connection session.
type ActiveSessionRecord struct {
	ID              string                               `json:"id"`
	ConnectionID    string                               `json:"connection_id"`
	ConnectionName  string                               `json:"connection_name,omitempty"`
	UserID          string                               `json:"user_id"`
	UserName        string                               `json:"user_name,omitempty"`
	TeamID          *string                              `json:"team_id,omitempty"`
	ProtocolID      string                               `json:"protocol_id"`
	DescriptorID    string                               `json:"descriptor_id,omitempty"`
	StartedAt       time.Time                            `json:"started_at"`
	LastSeenAt      time.Time                            `json:"last_seen_at"`
	Host            string                               `json:"host,omitempty"`
	Port            int                                  `json:"port,omitempty"`
	Metadata        map[string]any                       `json:"metadata,omitempty"`
	Template        map[string]any                       `json:"template,omitempty"`
	Capabilities    map[string]any                       `json:"capabilities,omitempty"`
	ConcurrentLimit int                                  `json:"concurrent_limit,omitempty"`
	OwnerUserID     string                               `json:"owner_user_id,omitempty"`
	OwnerUserName   string                               `json:"owner_user_name,omitempty"`
	Participants    map[string]*ActiveSessionParticipant `json:"participants,omitempty"`
	WriteHolder     string                               `json:"write_holder,omitempty"`
	chatBuffer      []ActiveSessionChatMessage
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
	mu               sync.RWMutex
	sessions         map[string]*ActiveSessionRecord
	userConnIndex    map[string]string
	connectionCounts map[string]int
	handles          map[string]drivers.SessionHandle
	hub              *realtime.Hub
	timeNow          func() time.Time
}

// NewActiveSessionService constructs an ActiveSessionService backed by the supplied realtime hub.
func NewActiveSessionService(hub *realtime.Hub) *ActiveSessionService {
	return &ActiveSessionService{
		sessions:         make(map[string]*ActiveSessionRecord),
		userConnIndex:    make(map[string]string),
		connectionCounts: make(map[string]int),
		handles:          make(map[string]drivers.SessionHandle),
		hub:              hub,
		timeNow:          time.Now,
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

	if session.OwnerUserID == "" {
		session.OwnerUserID = session.UserID
	}
	if session.OwnerUserName == "" {
		session.OwnerUserName = session.UserName
	}
	if session.Participants == nil {
		session.Participants = make(map[string]*ActiveSessionParticipant)
	}
	if ownerID := strings.TrimSpace(session.OwnerUserID); ownerID != "" {
		if existingOwner, ok := session.Participants[ownerID]; ok {
			if existingOwner.Role == "" {
				existingOwner.Role = "owner"
			}
			if existingOwner.AccessMode == "" {
				existingOwner.AccessMode = "write"
			}
			if existingOwner.JoinedAt.IsZero() {
				existingOwner.JoinedAt = session.StartedAt
			}
		} else {
			session.Participants[ownerID] = &ActiveSessionParticipant{
				SessionID:  session.ID,
				UserID:     ownerID,
				UserName:   session.OwnerUserName,
				Role:       "owner",
				AccessMode: "write",
				JoinedAt:   session.StartedAt,
			}
		}
		session.WriteHolder = ownerID
	}

	if session.ConcurrentLimit < 0 {
		session.ConcurrentLimit = 0
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

	limit := record.ConcurrentLimit
	if limit < 0 {
		limit = 0
	}
	record.ConcurrentLimit = limit
	currentCount := s.connectionCounts[record.ConnectionID]
	if limit > 0 && currentCount >= limit {
		return &ConcurrentLimitError{
			ConnectionID: record.ConnectionID,
			Limit:        limit,
			Reason:       ConcurrentLimitReasonReached,
		}
	}

	s.sessions[record.ID] = record
	s.userConnIndex[indexKey] = record.ID
	s.connectionCounts[record.ConnectionID] = currentCount + 1

	s.broadcastEvent("session.opened", record)

	return nil
}

// AddParticipant registers a participant with the active session and emits a realtime event.
func (s *ActiveSessionService) AddParticipant(sessionID string, participant ActiveSessionParticipant) (ActiveSessionParticipant, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ActiveSessionParticipant{}, errors.New("active session: session id is required")
	}
	userID := strings.TrimSpace(participant.UserID)
	if userID == "" {
		return ActiveSessionParticipant{}, errors.New("active session: participant user id is required")
	}

	if participant.Role == "" {
		participant.Role = "participant"
	}
	if participant.AccessMode == "" {
		participant.AccessMode = "read"
	}
	if participant.SessionID == "" {
		participant.SessionID = sessionID
	}
	if participant.JoinedAt.IsZero() {
		participant.JoinedAt = s.timeNow()
	}

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, fmt.Errorf("active session: session %s not found", sessionID)
	}
	if record.Participants == nil {
		record.Participants = make(map[string]*ActiveSessionParticipant)
	}
	candidate := participant
	record.Participants[userID] = &candidate
	if participant.AccessMode == "write" {
		s.setWriteHolderLocked(record, userID)
	}
	s.mu.Unlock()

	s.broadcastEvent("session.participant_joined", map[string]any{
		"session_id":  sessionID,
		"user_id":     participant.UserID,
		"user_name":   participant.UserName,
		"role":        participant.Role,
		"access_mode": participant.AccessMode,
		"joined_at":   participant.JoinedAt,
	})

	return participant, nil
}

// RemoveParticipant ejects a participant from the session, emitting relevant events.
func (s *ActiveSessionService) RemoveParticipant(sessionID, userID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	userID = strings.TrimSpace(userID)
	if sessionID == "" || userID == "" {
		return false
	}

	var (
		removed   *ActiveSessionParticipant
		newWriter string
	)

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if exists && record.Participants != nil {
		if participant, ok := record.Participants[userID]; ok && participant != nil {
			removed = cloneParticipant(participant)
			delete(record.Participants, userID)
		}
		if removed != nil && record.WriteHolder == userID {
			if owner := strings.TrimSpace(record.OwnerUserID); owner != "" && owner != userID {
				newWriter = owner
				s.setWriteHolderLocked(record, owner)
			} else {
				record.WriteHolder = ""
			}
		}
	}
	s.mu.Unlock()

	if removed == nil {
		return false
	}

	s.broadcastEvent("session.participant_left", map[string]any{
		"session_id": sessionID,
		"user_id":    removed.UserID,
		"user_name":  removed.UserName,
		"role":       removed.Role,
	})

	if newWriter != "" {
		s.broadcastEvent("session.write_granted", map[string]any{
			"session_id": sessionID,
			"user_id":    newWriter,
			"user_name":  s.participantName(sessionID, newWriter),
		})
	}

	return true
}

// GrantWriteAccess assigns write control to the specified participant.
func (s *ActiveSessionService) GrantWriteAccess(sessionID, userID string) (ActiveSessionParticipant, error) {
	sessionID = strings.TrimSpace(sessionID)
	userID = strings.TrimSpace(userID)
	if sessionID == "" || userID == "" {
		return ActiveSessionParticipant{}, errors.New("active session: session and user id are required")
	}

	var granted *ActiveSessionParticipant

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, fmt.Errorf("active session: session %s not found", sessionID)
	}
	if record.Participants == nil {
		record.Participants = make(map[string]*ActiveSessionParticipant)
	}
	if participant, ok := record.Participants[userID]; ok && participant != nil {
		s.setWriteHolderLocked(record, userID)
		granted = cloneParticipant(participant)
	} else {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, fmt.Errorf("active session: participant %s not found", userID)
	}
	s.mu.Unlock()

	s.broadcastEvent("session.write_granted", map[string]any{
		"session_id": sessionID,
		"user_id":    userID,
		"user_name":  granted.UserName,
	})

	return *granted, nil
}

// RelinquishWriteAccess releases write control for the specified participant and optionally
// transfers it back to the session owner. Returns the updated participant and the new writer, if any.
func (s *ActiveSessionService) RelinquishWriteAccess(sessionID, userID string) (ActiveSessionParticipant, *ActiveSessionParticipant, error) {
	sessionID = strings.TrimSpace(sessionID)
	userID = strings.TrimSpace(userID)
	if sessionID == "" || userID == "" {
		return ActiveSessionParticipant{}, nil, errors.New("active session: session and user id are required")
	}

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, nil, fmt.Errorf("active session: session %s not found", sessionID)
	}
	if record.Participants == nil {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, nil, fmt.Errorf("active session: participant %s not found", userID)
	}
	participant, ok := record.Participants[userID]
	if !ok || participant == nil {
		s.mu.Unlock()
		return ActiveSessionParticipant{}, nil, fmt.Errorf("active session: participant %s not found", userID)
	}

	// If the participant is not the current write holder, no changes are required.
	if !strings.EqualFold(record.WriteHolder, userID) {
		updated := cloneParticipant(participant)
		s.mu.Unlock()
		return *updated, nil, nil
	}

	ownerID := strings.TrimSpace(record.OwnerUserID)
	newWriterID := ""
	if ownerID != "" && !strings.EqualFold(ownerID, userID) {
		newWriterID = ownerID
	}

	s.setWriteHolderLocked(record, newWriterID)

	updatedParticipant := cloneParticipant(record.Participants[userID])

	var newWriter *ActiveSessionParticipant
	if newWriterID != "" {
		if candidate, exists := record.Participants[newWriterID]; exists && candidate != nil {
			newWriter = cloneParticipant(candidate)
		}
	}

	s.mu.Unlock()

	payload := map[string]any{
		"session_id": sessionID,
		"user_id":    "",
		"user_name":  "",
	}
	if newWriter != nil {
		payload["user_id"] = newWriter.UserID
		payload["user_name"] = newWriter.UserName
	}
	s.broadcastEvent("session.write_granted", payload)

	return *updatedParticipant, newWriter, nil
}

// GetSession returns a copy of the active session record if present.
func (s *ActiveSessionService) GetSession(sessionID string) (*ActiveSessionRecord, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	return cloneSessionRecord(record), true
}

// AppendChatMessage appends a chat message to the in-memory buffer and emits realtime updates.
func (s *ActiveSessionService) AppendChatMessage(sessionID string, message ActiveSessionChatMessage) (ActiveSessionChatMessage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ActiveSessionChatMessage{}, errors.New("active session: session id is required")
	}
	if strings.TrimSpace(message.AuthorID) == "" {
		return ActiveSessionChatMessage{}, errors.New("active session: message author id is required")
	}
	if strings.TrimSpace(message.Content) == "" {
		return ActiveSessionChatMessage{}, errors.New("active session: message content is required")
	}
	if message.SessionID == "" {
		message.SessionID = sessionID
	}
	if message.MessageID == "" {
		message.MessageID = uuid.NewString()
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = s.timeNow()
	}

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return ActiveSessionChatMessage{}, fmt.Errorf("active session: session %s not found", sessionID)
	}
	record.chatBuffer = append(record.chatBuffer, message)
	if len(record.chatBuffer) > defaultChatBufferSize {
		record.chatBuffer = record.chatBuffer[len(record.chatBuffer)-defaultChatBufferSize:]
	}
	s.mu.Unlock()

	s.broadcastEvent("session.chat_posted", map[string]any{
		"session_id": sessionID,
		"message_id": message.MessageID,
		"author_id":  message.AuthorID,
		"author":     message.Author,
		"content":    message.Content,
		"created_at": message.CreatedAt,
	})

	return message, nil
}

// ConsumeChatBuffer returns the buffered chat messages and clears the buffer.
func (s *ActiveSessionService) ConsumeChatBuffer(sessionID string) []ActiveSessionChatMessage {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.sessions[sessionID]
	if !exists || len(record.chatBuffer) == 0 {
		return nil
	}

	buffer := make([]ActiveSessionChatMessage, len(record.chatBuffer))
	copy(buffer, record.chatBuffer)
	record.chatBuffer = record.chatBuffer[:0]
	return buffer
}

// AckChatMessage removes a chat message from the pending buffer once persisted.
func (s *ActiveSessionService) AckChatMessage(sessionID, messageID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	messageID = strings.TrimSpace(messageID)
	if sessionID == "" || messageID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.sessions[sessionID]
	if !exists || len(record.chatBuffer) == 0 {
		return false
	}

	for idx, msg := range record.chatBuffer {
		if msg.MessageID == messageID {
			record.chatBuffer = append(record.chatBuffer[:idx], record.chatBuffer[idx+1:]...)
			return true
		}
	}
	return false
}

// UnregisterSession removes the session from the registry and notifies listeners.
func (s *ActiveSessionService) UnregisterSession(sessionID string) {
	if sessionID == "" {
		return
	}

	var handle drivers.SessionHandle

	s.mu.Lock()
	record, exists := s.sessions[sessionID]
	if exists {
		delete(s.sessions, sessionID)
		delete(s.userConnIndex, userConnectionKey(record.UserID, record.ConnectionID))
		if count := s.connectionCounts[record.ConnectionID]; count > 1 {
			s.connectionCounts[record.ConnectionID] = count - 1
		} else {
			delete(s.connectionCounts, record.ConnectionID)
		}
		if existing, ok := s.handles[sessionID]; ok {
			handle = existing
			delete(s.handles, sessionID)
		}
	}
	s.mu.Unlock()

	if !exists {
		if handle != nil {
			_ = handle.Close(context.Background())
		}
		return
	}

	s.broadcastEvent("session.closed", map[string]any{
		"id":            record.ID,
		"connection_id": record.ConnectionID,
		"user_id":       record.UserID,
	})

	if handle != nil {
		_ = handle.Close(context.Background())
	}
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

// AttachHandle associates a driver session handle with the active session identifier.
func (s *ActiveSessionService) AttachHandle(sessionID string, handle drivers.SessionHandle) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || handle == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.handles[sessionID]; ok && existing != nil && existing != handle {
		_ = existing.Close(context.Background())
	}
	s.handles[sessionID] = handle
}

// CheckoutHandle retrieves and removes a handle associated with the session.
func (s *ActiveSessionService) CheckoutHandle(sessionID string) (drivers.SessionHandle, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	handle, ok := s.handles[sessionID]
	if !ok || handle == nil {
		return nil, false
	}

	delete(s.handles, sessionID)
	return handle, true
}

// PeekHandle returns the handle associated with the session without removing it.
func (s *ActiveSessionService) PeekHandle(sessionID string) (drivers.SessionHandle, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	handle, ok := s.handles[sessionID]
	if !ok || handle == nil {
		return nil, false
	}
	return handle, true
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
			if count := s.connectionCounts[record.ConnectionID]; count > 1 {
				s.connectionCounts[record.ConnectionID] = count - 1
			} else {
				delete(s.connectionCounts, record.ConnectionID)
			}
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

func (s *ActiveSessionService) participantName(sessionID, userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.sessions[sessionID]
	if !ok || record.Participants == nil {
		return ""
	}
	if participant, exists := record.Participants[userID]; exists && participant != nil {
		return participant.UserName
	}
	return ""
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
	clone.ConcurrentLimit = session.ConcurrentLimit
	if session.Metadata != nil {
		meta := make(map[string]any, len(session.Metadata))
		for key, value := range session.Metadata {
			meta[key] = value
		}
		clone.Metadata = meta
	}
	if session.Template != nil {
		template := make(map[string]any, len(session.Template))
		for key, value := range session.Template {
			template[key] = value
		}
		clone.Template = template
	}
	if session.Capabilities != nil {
		caps := make(map[string]any, len(session.Capabilities))
		for key, value := range session.Capabilities {
			caps[key] = value
		}
		clone.Capabilities = caps
	}
	if session.Participants != nil {
		clone.Participants = make(map[string]*ActiveSessionParticipant, len(session.Participants))
		for key, participant := range session.Participants {
			clone.Participants[key] = cloneParticipant(participant)
		}
	}
	if len(session.chatBuffer) > 0 {
		clone.chatBuffer = make([]ActiveSessionChatMessage, len(session.chatBuffer))
		copy(clone.chatBuffer, session.chatBuffer)
	}
	return &clone
}

func userConnectionKey(userID, connectionID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(userID), strings.TrimSpace(connectionID))
}

func cloneParticipant(participant *ActiveSessionParticipant) *ActiveSessionParticipant {
	if participant == nil {
		return nil
	}
	clone := *participant
	return &clone
}

func (s *ActiveSessionService) setWriteHolderLocked(record *ActiveSessionRecord, userID string) {
	if record == nil {
		return
	}
	if record.Participants == nil {
		record.Participants = make(map[string]*ActiveSessionParticipant)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		for _, participant := range record.Participants {
			if participant != nil {
				participant.AccessMode = "read"
			}
		}
		record.WriteHolder = ""
		return
	}

	for key, participant := range record.Participants {
		if participant == nil {
			continue
		}
		if strings.EqualFold(key, userID) {
			participant.AccessMode = "write"
			record.WriteHolder = participant.UserID
		} else if participant.AccessMode == "write" {
			participant.AccessMode = "read"
		}
	}

	if record.WriteHolder == "" {
		if participant, ok := record.Participants[userID]; ok && participant != nil {
			participant.AccessMode = "write"
			record.WriteHolder = participant.UserID
		}
	}
}
