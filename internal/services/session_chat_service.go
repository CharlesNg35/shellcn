package services

import (
	"context"
	"errors"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
)

const maxChatMessageLength = 4000

// ChatMessageParams carries the payload required to post a chat message.
type ChatMessageParams struct {
	SessionID string
	AuthorID  string
	Author    string
	Content   string
}

// SessionChatService persists chat messages and relays them through the active session service.
type SessionChatService struct {
	db      *gorm.DB
	active  *ActiveSessionService
	timeNow func() time.Time
}

// NewSessionChatService constructs a chat service once database and active session dependencies are supplied.
func NewSessionChatService(db *gorm.DB, active *ActiveSessionService) (*SessionChatService, error) {
	if db == nil {
		return nil, errors.New("session chat service: db is required")
	}
	if active == nil {
		return nil, errors.New("session chat service: active session service is required")
	}
	return &SessionChatService{
		db:      db,
		active:  active,
		timeNow: time.Now,
	}, nil
}

// PostMessage sanitises, persists, and broadcasts a chat message for the supplied session.
func (s *SessionChatService) PostMessage(ctx context.Context, params ChatMessageParams) (ActiveSessionChatMessage, error) {
	if s == nil {
		return ActiveSessionChatMessage{}, errors.New("session chat service: service not initialised")
	}
	ctx = ensureContext(ctx)

	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return ActiveSessionChatMessage{}, errors.New("session chat service: session id is required")
	}
	authorID := strings.TrimSpace(params.AuthorID)
	if authorID == "" {
		return ActiveSessionChatMessage{}, errors.New("session chat service: author id is required")
	}

	content := strings.TrimSpace(params.Content)
	if content == "" {
		return ActiveSessionChatMessage{}, errors.New("session chat service: message content is required")
	}
	if utf8.RuneCountInString(content) > maxChatMessageLength {
		return ActiveSessionChatMessage{}, errors.New("session chat service: message content exceeds maximum length")
	}
	sanitized := html.EscapeString(content)

	message := ActiveSessionChatMessage{
		MessageID: uuid.NewString(),
		SessionID: sessionID,
		AuthorID:  authorID,
		Author:    strings.TrimSpace(params.Author),
		Content:   sanitized,
		CreatedAt: s.timeNow(),
	}

	if err := s.PersistMessages(ctx, sessionID, []ActiveSessionChatMessage{message}); err != nil {
		return ActiveSessionChatMessage{}, err
	}

	if _, exists := s.active.GetSession(sessionID); exists {
		if _, err := s.active.AppendChatMessage(sessionID, message); err == nil {
			s.active.AckChatMessage(sessionID, message.MessageID)
		}
	}

	return message, nil
}

// PersistMessages stores the supplied chat messages for the session.
func (s *SessionChatService) PersistMessages(ctx context.Context, sessionID string, messages []ActiveSessionChatMessage) error {
	if s == nil {
		return errors.New("session chat service: service not initialised")
	}
	ctx = ensureContext(ctx)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session chat service: session id is required")
	}

	if len(messages) == 0 {
		return nil
	}

	records := make([]models.ConnectionSessionMessage, 0, len(messages))
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" || strings.TrimSpace(msg.AuthorID) == "" {
			continue
		}
		record := models.ConnectionSessionMessage{
			BaseModel: models.BaseModel{ID: strings.TrimSpace(msg.MessageID)},
			SessionID: sessionID,
			AuthorID:  strings.TrimSpace(msg.AuthorID),
			Content:   msg.Content,
		}
		if !msg.CreatedAt.IsZero() {
			record.CreatedAt = msg.CreatedAt
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}

// ListMessages returns persisted chat messages for the supplied session ordered chronologically.
func (s *SessionChatService) ListMessages(ctx context.Context, sessionID string, limit int, before time.Time) ([]models.ConnectionSessionMessage, error) {
	if s == nil {
		return nil, errors.New("session chat service: service not initialised")
	}
	ctx = ensureContext(ctx)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session chat service: session id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := s.db.WithContext(ctx).
		Model(&models.ConnectionSessionMessage{}).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit)

	if !before.IsZero() {
		query = query.Where("created_at < ?", before)
	}

	var rows []models.ConnectionSessionMessage
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	// Reverse to chronological order
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

	return rows, nil
}
