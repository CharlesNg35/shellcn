package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/metrics"
)

// DefaultRefreshTokenTTL is the fallback refresh token lifetime.
const DefaultRefreshTokenTTL = 30 * 24 * time.Hour

// SessionConfig describes tunable behaviour for the SessionService.
type SessionConfig struct {
	RefreshTokenTTL time.Duration
	RefreshLength   int
	Clock           func() time.Time
	Cache           SessionCache
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

// AuthSubject represents the authenticated principal for whom a session is being issued.
type AuthSubject struct {
	UserID     string
	Provider   string
	ExternalID string
	Email      string
	Claims     map[string]any
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

var errSessionCacheMiss = errors.New("session cache miss")

// SessionCache represents a cache backend for session objects keyed by refresh token.
type SessionCache interface {
	Get(ctx context.Context, refreshToken string) (*models.Session, error)
	Set(ctx context.Context, session *models.Session, ttl time.Duration) error
	Delete(ctx context.Context, refreshToken string) error
}

// SessionService manages creation, rotation, and revocation of user sessions.
type SessionService struct {
	db         *gorm.DB
	jwt        *JWTService
	refreshTTL time.Duration
	tokenLen   int
	now        func() time.Time
	cache      SessionCache
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
		cache:      cfg.Cache,
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

	metrics.ActiveSessions.Inc()

	accessToken, err := s.jwt.GenerateAccessToken(AccessTokenInput{
		UserID:    userID,
		SessionID: session.ID,
		Metadata:  cloneMetadata(meta.Claims),
	})
	if err != nil {
		return TokenPair{}, nil, fmt.Errorf("session service: generate access token: %w", err)
	}

	if s.cache != nil {
		if err := s.cache.Set(context.Background(), session, s.refreshTTL); err != nil {
			// Cache failures are non-fatal; proceed without returning an error.
		}
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, session, nil
}

// CreateForSubject issues a session for the supplied authenticated subject while enriching metadata with SSO attributes.
func (s *SessionService) CreateForSubject(subject AuthSubject, meta SessionMetadata) (TokenPair, *models.Session, error) {
	if strings.TrimSpace(subject.UserID) == "" {
		return TokenPair{}, nil, errors.New("session service: subject user id is required")
	}

	merged := make(map[string]any)
	for k, v := range meta.Claims {
		if k != "" {
			merged[k] = v
		}
	}
	for k, v := range subject.Claims {
		if k != "" {
			merged[k] = v
		}
	}

	if provider := strings.TrimSpace(subject.Provider); provider != "" {
		merged["sso_provider"] = provider
	}
	if externalID := strings.TrimSpace(subject.ExternalID); externalID != "" {
		merged["sso_subject"] = externalID
	}
	if email := strings.TrimSpace(subject.Email); email != "" {
		merged["sso_email"] = strings.ToLower(email)
	}

	meta.Claims = merged
	return s.CreateSession(subject.UserID, meta)
}

// RefreshSession rotates the refresh token and issues a new access token.
func (s *SessionService) RefreshSession(refreshToken string) (TokenPair, *models.Session, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenPair{}, nil, ErrSessionInvalidToken
	}

	var session models.Session
	var err error
	var cacheHit bool

	if s.cache != nil {
		if cached, cacheErr := s.cache.Get(context.Background(), refreshToken); cacheErr == nil && cached != nil {
			session = *cached
			cacheHit = true
		} else if cacheErr != nil && !errors.Is(cacheErr, errSessionCacheMiss) {
			cacheHit = false
		}
	}

	if !cacheHit {
		err = s.db.Where("refresh_token = ?", refreshToken).Take(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TokenPair{}, nil, ErrSessionNotFound
		}
		if err != nil {
			return TokenPair{}, nil, fmt.Errorf("session service: find session: %w", err)
		}
		if s.cache != nil {
			if ttl := time.Until(session.ExpiresAt); ttl > 0 {
				_ = s.cache.Set(context.Background(), &session, ttl)
			}
		}
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

	if s.cache != nil {
		_ = s.cache.Delete(context.Background(), refreshToken)
		ttl := time.Until(session.ExpiresAt)
		if ttl <= 0 {
			ttl = s.refreshTTL
		}
		_ = s.cache.Set(context.Background(), &session, ttl)
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

	var tokenToDelete string
	if s.cache != nil {
		var session models.Session
		if err := s.db.Select("refresh_token").Take(&session, "id = ?", sessionID).Error; err == nil {
			tokenToDelete = session.RefreshToken
		}
	}

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

	if s.cache != nil && tokenToDelete != "" {
		_ = s.cache.Delete(context.Background(), tokenToDelete)
	}

	metrics.ActiveSessions.Sub(float64(result.RowsAffected))

	return nil
}

// CleanupExpired removes expired sessions and updates active session metrics accordingly.
func (s *SessionService) CleanupExpired(ctx context.Context) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	now := s.now()

	var activeExpired int64
	if err := s.db.WithContext(ctx).
		Model(&models.Session{}).
		Where("expires_at < ? AND revoked_at IS NULL", now).
		Count(&activeExpired).Error; err != nil {
		return 0, fmt.Errorf("session service: count expired sessions: %w", err)
	}

	result := s.db.WithContext(ctx).
		Where("expires_at < ?", now).
		Or("revoked_at IS NOT NULL").
		Delete(&models.Session{})
	if result.Error != nil {
		return 0, fmt.Errorf("session service: cleanup expired sessions: %w", result.Error)
	}

	if s.cache != nil {
		var tokens []string
		if err := s.db.WithContext(ctx).
			Model(&models.Session{}).
			Where("expires_at < ?", now).
			Or("revoked_at IS NOT NULL").
			Pluck("refresh_token", &tokens).Error; err == nil {
			for _, token := range tokens {
				if strings.TrimSpace(token) == "" {
					continue
				}
				_ = s.cache.Delete(ctx, token)
			}
		}
	}

	if activeExpired > 0 {
		metrics.ActiveSessions.Sub(float64(activeExpired))
	}

	return result.RowsAffected, nil
}

// RevokeUserSessions revokes every active session belonging to a user.
func (s *SessionService) RevokeUserSessions(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrSessionInvalidToken
	}

	now := s.now()
	var tokens []string
	if s.cache != nil {
		if err := s.db.
			Model(&models.Session{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Pluck("refresh_token", &tokens).Error; err != nil {
			tokens = nil
		}
	}

	result := s.db.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		metrics.ActiveSessions.Sub(float64(result.RowsAffected))
	}

	if s.cache != nil {
		for _, token := range tokens {
			if strings.TrimSpace(token) == "" {
				continue
			}
			_ = s.cache.Delete(context.Background(), token)
		}
	}
	return nil
}
