package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/pkg/crypto"
)

var (
	errStateExpired = errors.New("sso state: expired")
	errStateInvalid = errors.New("sso state: invalid")
)

// StateCodec encodes and decodes SSO state payloads used during external auth flows.
type StateCodec struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// StatePayload captures data required to validate the callback and resume the login flow.
type StatePayload struct {
	Provider   string    `json:"p"`
	ReturnURL  string    `json:"r"`
	ErrorURL   string    `json:"e"`
	Nonce      string    `json:"n"`
	PKCE       string    `json:"k"`
	IssuedAt   time.Time `json:"iat"`
	AutoCreate bool      `json:"ac"`
	RequestID  string    `json:"req"`
}

// NewStateCodec constructs a StateCodec using the provided symmetric encryption key and lifetime.
func NewStateCodec(key []byte, ttl time.Duration, now func() time.Time) (*StateCodec, error) {
	length := len(key)
	if length != 16 && length != 24 && length != 32 {
		return nil, fmt.Errorf("sso state: key must be 16, 24, or 32 bytes, got %d", length)
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &StateCodec{
		key: key,
		ttl: ttl,
		now: now,
	}, nil
}

// Encode encrypts the supplied payload into a compact state string.
func (c *StateCodec) Encode(payload StatePayload) (string, error) {
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	if payload.Provider == "" {
		return "", errors.New("sso state: provider is required")
	}
	payload.IssuedAt = c.now().UTC()

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("sso state: marshal payload: %w", err)
	}

	encoded, err := crypto.Encrypt(raw, c.key)
	if err != nil {
		return "", fmt.Errorf("sso state: encrypt payload: %w", err)
	}

	return encoded, nil
}

// Decode decrypts the state string back into a payload while enforcing expiry.
func (c *StateCodec) Decode(token string) (StatePayload, error) {
	var payload StatePayload
	if strings.TrimSpace(token) == "" {
		return payload, errStateInvalid
	}

	raw, err := crypto.Decrypt(token, c.key)
	if err != nil {
		return payload, errStateInvalid
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, errStateInvalid
	}

	if payload.Provider == "" || payload.IssuedAt.IsZero() {
		return payload, errStateInvalid
	}

	if c.now().UTC().After(payload.IssuedAt.Add(c.ttl)) {
		return payload, errStateExpired
	}

	return payload, nil
}
