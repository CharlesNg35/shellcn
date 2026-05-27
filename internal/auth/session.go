package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// SessionCookieName carries the signed stateless browser session JWT.
	SessionCookieName = "shellcn_session"
	// CSRFHeader is where state-changing HTTP requests echo the CSRF token.
	CSRFHeader = "X-CSRF-Token"
	// DefaultSessionTTL is how long a platform session lives.
	DefaultSessionTTL = 24 * time.Hour
	sessionIssuer     = "shellcn"
)

// Session is one authenticated browser session.
type Session struct {
	ID             string
	UserID         string
	CSRFToken      string
	SessionVersion int
	ExpiresAt      time.Time
}

type sessionClaims struct {
	CSRFToken      string `json:"csrf"`
	SessionVersion int    `json:"ver"`
	jwt.RegisteredClaims
}

// SessionManager signs and verifies stateless browser session JWTs.
type SessionManager struct {
	key     []byte
	ttl     time.Duration
	mu      sync.Mutex
	revoked map[string]time.Time
}

// NewSessionManager returns a manager with an ephemeral signing key. Production
// code should use NewSessionManagerWithKey so sessions survive process restarts.
func NewSessionManager(ttl time.Duration) *SessionManager {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return NewSessionManagerWithKey(ttl, key)
}

// NewSessionManagerWithKey returns a JWT session manager with a stable HMAC key.
func NewSessionManagerWithKey(ttl time.Duration, key []byte) *SessionManager {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	if len(key) < 32 {
		panic("auth: JWT signing key must be at least 32 bytes")
	}
	return &SessionManager{key: append([]byte(nil), key...), ttl: ttl, revoked: map[string]time.Time{}}
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Create starts a new stateless session for userID, returning its signed JWT.
func (m *SessionManager) Create(userID string, sessionVersion ...int) Session {
	now := time.Now()
	version := 0
	if len(sessionVersion) > 0 {
		version = sessionVersion[0]
	}
	s := Session{
		UserID:         userID,
		CSRFToken:      randomToken(),
		SessionVersion: version,
		ExpiresAt:      now.Add(m.ttl),
	}
	claims := sessionClaims{
		CSRFToken:      s.CSRFToken,
		SessionVersion: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    sessionIssuer,
			Subject:   userID,
			ID:        randomToken(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(s.ExpiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.key)
	if err != nil {
		panic("auth: sign session JWT: " + err.Error())
	}
	s.ID = token
	return s
}

// Get validates a signed session JWT.
func (m *SessionManager) Get(tokenString string) (Session, bool) {
	if m.isRevoked(tokenString) {
		return Session{}, false
	}
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.key, nil
	}, jwt.WithIssuer(sessionIssuer))
	if err != nil || !token.Valid {
		return Session{}, false
	}
	if claims.Subject == "" || claims.CSRFToken == "" || claims.ExpiresAt == nil {
		return Session{}, false
	}
	return Session{
		ID:             tokenString,
		UserID:         claims.Subject,
		CSRFToken:      claims.CSRFToken,
		SessionVersion: claims.SessionVersion,
		ExpiresAt:      claims.ExpiresAt.Time,
	}, true
}

// Destroy revokes one browser session token for the remainder of its lifetime.
func (m *SessionManager) Destroy(tokenString string) {
	if tokenString == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneRevokedLocked(time.Now())
	m.revoked[tokenString] = time.Now().Add(m.ttl)
}

func (m *SessionManager) isRevoked(tokenString string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	exp, ok := m.revoked[tokenString]
	if !ok {
		m.pruneRevokedLocked(now)
		return false
	}
	if now.After(exp) {
		delete(m.revoked, tokenString)
		return false
	}
	return true
}

func (m *SessionManager) pruneRevokedLocked(now time.Time) {
	for token, exp := range m.revoked {
		if now.After(exp) {
			delete(m.revoked, token)
		}
	}
}

// ValidateCSRF reports whether the request carries the session's CSRF token.
func (s Session) ValidateCSRF(r *http.Request) bool {
	got := r.Header.Get(CSRFHeader)
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRFToken)) == 1
}

// SetSessionCookie writes the HttpOnly, SameSite=Lax session cookie. Secure is
// set when the request is served over TLS.
func SetSessionCookie(w http.ResponseWriter, s Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    s.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.ExpiresAt,
	})
}

// ClearSessionCookie expires the session cookie on logout.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
