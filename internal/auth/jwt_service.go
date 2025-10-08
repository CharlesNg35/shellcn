package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultAccessTokenTTL defines the fallback validity period for access tokens.
const DefaultAccessTokenTTL = 15 * time.Minute

// JWTConfig bundles the configuration required to build a JWTService.
type JWTConfig struct {
	Secret         string
	Issuer         string
	AccessTokenTTL time.Duration
	Clock          func() time.Time
}

// Claims represents the custom claims embedded in issued JWTs.
type Claims struct {
	UserID    string         `json:"uid"`
	SessionID string         `json:"sid,omitempty"`
	Metadata  map[string]any `json:"meta,omitempty"`
	jwt.RegisteredClaims
}

// AccessTokenInput holds the parameters used when generating a new access token.
type AccessTokenInput struct {
	UserID    string
	SessionID string
	Audience  []string
	Metadata  map[string]any
}

// JWTService is responsible for issuing and validating JSON Web Tokens.
type JWTService struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewJWTService constructs a JWTService instance when provided with the required configuration.
func NewJWTService(cfg JWTConfig) (*JWTService, error) {
	if cfg.Secret == "" {
		return nil, errors.New("jwt: secret must be provided")
	}

	ttl := cfg.AccessTokenTTL
	if ttl <= 0 {
		ttl = DefaultAccessTokenTTL
	}

	now := time.Now
	if cfg.Clock != nil {
		now = cfg.Clock
	}

	return &JWTService{
		secret: []byte(cfg.Secret),
		issuer: cfg.Issuer,
		ttl:    ttl,
		now:    now,
	}, nil
}

// GenerateAccessToken issues a signed JWT containing the supplied claims.
func (s *JWTService) GenerateAccessToken(input AccessTokenInput) (string, error) {
	if input.UserID == "" {
		return "", errors.New("jwt: user id is required")
	}

	now := s.now()
	expiresAt := now.Add(s.ttl)

	claims := &Claims{
		UserID:    input.UserID,
		SessionID: input.SessionID,
		Metadata:  cloneMetadata(input.Metadata),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   input.UserID,
			Issuer:    s.issuer,
			Audience:  input.Audience,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	if input.SessionID != "" {
		claims.ID = input.SessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}

	return signed, nil
}

// ValidateAccessToken parses and validates a signed JWT, returning the application claims.
func (s *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("jwt: token string is empty")
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(s.now),
	)

	var claims Claims
	_, err := parser.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt: parse token: %w", err)
	}

	if s.issuer != "" && claims.Issuer != s.issuer {
		return nil, errors.New("jwt: invalid issuer")
	}

	if claims.UserID == "" {
		return nil, errors.New("jwt: missing user id claim")
	}

	return &claims, nil
}

// cloneMetadata guards against accidental external mutation of stored metadata.
func cloneMetadata(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}

	cpy := make(map[string]any, len(meta))
	for k, v := range meta {
		cpy[k] = v
	}
	return cpy
}
