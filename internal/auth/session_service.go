package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

// DefaultRefreshTokenTTL is the fallback refresh token lifetime.
const DefaultRefreshTokenTTL = 30 * 24 * time.Hour

// SessionConfig describes tunable behaviour for the SessionService.
type SessionConfig struct {
	RefreshTokenTTL time.Duration
	RefreshLength   int
	Clock           func() time.Time
}

// SessionMetadata captures contextual information about the client.
type SessionMetadata struct {
	IPAddress string
	UserAgent string
	Device    string
	Claims    map[string]any
}

// TokenPair represents an access token and refresh token pair.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

var (
	// ErrSessionNotFound indicates that no session matches the provided token or identifier.
	ErrSessionNotFound = errors.New("session: not found")
	// ErrSessionRevoked marks a session that has been revoked by the user or administrators.
	ErrSessionRevoked = errors.New("session: revoked")
	// ErrSessionExpired signals that a refresh token has reached its expiry.
	ErrSessionExpired = errors.New("session: expired")
	// ErrSessionInvalidToken is returned when the supplied refresh token is malformed.
	ErrSessionInvalidToken = errors.New("session: invalid token")
)

// SessionService manages creation, rotation, and revocation of user sessions.
type SessionService struct {
	db         *gorm.DB
	jwt        *JWTService
	refreshTTL time.Duration
	tokenLen   int
	now        func() time.Time
}

// NewSessionService constructs a session manager backed by the provided database and JWT service.
func NewSessionService(db *gorm.DB, jwtService *JWTService, cfg SessionConfig) (*SessionService, error) {
	if db == nil {
		return nil, errors.New("session service: db is required")
	}
	if jwtService == nil {
		return nil, errors.New("session service: jwt service is required")
	}

	ttl := cfg.RefreshTokenTTL
	if ttl <= 0 {
		ttl = DefaultRefreshTokenTTL
	}

	length := cfg.RefreshLength
	if length <= 0 {
		length = 48
	}

	clock := time.Now
	if cfg.Clock != nil {
		clock = cfg.Clock
	}

	return &SessionService{
		db:         db,
		jwt:        jwtService,
		refreshTTL: ttl,
		tokenLen:   length,
		now:        clock,
	}, nil
}

// CreateSession generates a new session and issues a fresh token pair.
func (s *SessionService) CreateSession(userID string, meta SessionMetadata) (TokenPair, *models.Session, error) {
	if strings.TrimSpace(userID) == "" {
		return TokenPair{}, nil, errors.New("session service: user id is required")
	}

	refreshToken, err := crypto.GenerateToken(s.tokenLen)
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: generate refresh token: %w", err)
	}

	now := s.now()

	session := &models.Session{
		UserID:       userID,
		RefreshToken: refreshToken,
		IPAddress:    strings.TrimSpace(meta.IPAddress),
		UserAgent:    strings.TrimSpace(meta.UserAgent),
		DeviceName:   strings.TrimSpace(meta.Device),
		ExpiresAt:    now.Add(s.refreshTTL),
		LastUsedAt:   now,
	}

	if err := s.db.Create(session).Error; err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: create session: %w", err)
	}

	accessToken, err := s.jwt.GenerateAccessToken(AccessTokenInput{
		UserID:    userID,
		SessionID: session.ID,
		Metadata:  cloneMetadata(meta.Claims),
	})
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: generate access token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, session, nil
}

// RefreshSession rotates the refresh token and issues a new access token.
func (s *SessionService) RefreshSession(refreshToken string) (TokenPair, *models.Session, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenPair{}, nil, ErrSessionInvalidToken
	}

	var session models.Session
	err := s.db.Where("refresh_token = ?", refreshToken).Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TokenPair{}, nil, ErrSessionNotFound
	}
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: find session: %w", err)
	}

	now := s.now()

	if session.RevokedAt != nil {
		return TokenPair{}, nil, ErrSessionRevoked
	}

	if session.ExpiresAt.Before(now) {
		return TokenPair{}, nil, ErrSessionExpired
	}

	newRefresh, err := crypto.GenerateToken(s.tokenLen)
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: generate refresh token: %w", err)
	}

	updates := map[string]any{
		"refresh_token": newRefresh,
		"expires_at":    now.Add(s.refreshTTL),
		"last_used_at":  now,
	}

	if err := s.db.Model(&session).Updates(updates).Error; err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: update session: %w", err)
	}

	session.RefreshToken = newRefresh
	session.ExpiresAt = updates["expires_at"].(time.Time)
	session.LastUsedAt = now

	accessToken, err := s.jwt.GenerateAccessToken(AccessTokenInput{
		UserID:    session.UserID,
		SessionID: session.ID,
	})
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: generate access token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
	}, &session, nil
}

// RevokeSession marks a session as revoked, preventing further refresh operations.
func (s *SessionService) RevokeSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrSessionInvalidToken
	}

	now := s.now()

	result := s.db.Model(&models.Session{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]any{
			"revoked_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("session service: revoke session: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// RevokeUserSessions revokes every active session belonging to a user.
func (s *SessionService) RevokeUserSessions(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrSessionInvalidToken
	}

	now := s.now()
	return s.db.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}
