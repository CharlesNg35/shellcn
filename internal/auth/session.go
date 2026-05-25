package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const (
	// SessionCookieName carries the opaque platform session id.
	SessionCookieName = "shellcn_session"
	// CSRFHeader is where state-changing HTTP requests echo the CSRF token.
	CSRFHeader = "X-CSRF-Token"
	// DefaultSessionTTL is how long a platform session lives.
	DefaultSessionTTL = 24 * time.Hour
)

// Session is one authenticated browser session.
type Session struct {
	ID        string
	UserID    string
	CSRFToken string
	ExpiresAt time.Time
}

func (s Session) expired() bool { return time.Now().After(s.ExpiresAt) }

// SessionManager is an in-memory platform session registry. Sessions are
// revocable on logout; the registry is not shared across instances.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

// NewSessionManager returns a manager with the given TTL (0 = default).
func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionManager{sessions: make(map[string]Session), ttl: ttl}
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// A failing system CSPRNG must never yield a predictable session/CSRF
		// token; fail loudly rather than emit weak entropy.
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Create starts a new session for userID, returning it with a fresh CSRF token.
func (m *SessionManager) Create(userID string) Session {
	s := Session{
		ID:        randomToken(),
		UserID:    userID,
		CSRFToken: randomToken(),
		ExpiresAt: time.Now().Add(m.ttl),
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

// Get returns a live (non-expired) session by id.
func (m *SessionManager) Get(id string) (Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if s.expired() {
		m.Destroy(id)
		return Session{}, false
	}
	return s, true
}

// Destroy revokes a session.
func (m *SessionManager) Destroy(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
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
