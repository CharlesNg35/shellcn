package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/livelease"
)

// DefaultTicketTTL is the short lifetime of a WS ticket.
const DefaultTicketTTL = 30 * time.Second

const (
	ticketIssuer  = app.SessionIssuer
	ticketPurpose = "ws_ticket"
)

// ErrTicketInvalid is returned for any ticket failure (unknown, expired, used,
// or scope mismatch) — deliberately undifferentiated so it leaks nothing.
var ErrTicketInvalid = errors.New("auth: invalid ticket")

// TicketScope binds a ticket to exactly one resource + acting user. Binding the
// params means a ticket minted for one stream scope cannot be replayed against
// another.
type TicketScope struct {
	ConnectionID string
	RouteID      string
	Params       map[string]string
	UserID       string
}

type ticketClaims struct {
	Purpose string            `json:"purpose"`
	Params  map[string]string `json:"params,omitempty"`
	jwt.RegisteredClaims
}

type TicketStoreOptions struct {
	TTL        time.Duration
	SigningKey []byte
	Leases     livelease.LeaseRegistry
	Instance   livelease.InstanceRef
}

// TicketStore mints and redeems short-lived WS tickets. Browsers can't set
// Authorization on a WS upgrade, so tickets are mandatory.
type TicketStore struct {
	ttl      time.Duration
	key      []byte
	leases   livelease.LeaseRegistry
	instance livelease.InstanceRef
}

func NewTicketStore(opts TicketStoreOptions) *TicketStore {
	if opts.TTL <= 0 {
		opts.TTL = DefaultTicketTTL
	}
	if len(opts.SigningKey) < 32 {
		panic("auth: ticket signing key must be at least 32 bytes")
	}
	if opts.Leases == nil {
		panic("auth: ticket lease registry is required")
	}
	return &TicketStore{
		ttl:      opts.TTL,
		key:      append([]byte(nil), opts.SigningKey...),
		leases:   opts.Leases,
		instance: opts.Instance,
	}
}

// Mint issues a token bound to scope, expiring after the store's TTL.
func (s *TicketStore) Mint(scope TicketScope) (token string, expiresAt time.Time) {
	now := time.Now()
	expiresAt = now.Add(s.ttl)
	claims := ticketClaims{
		Purpose: ticketPurpose,
		Params:  cloneParams(scope.Params),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ticketIssuer,
			Subject:   scope.UserID,
			ID:        randomToken(),
			Audience:  []string{scope.ConnectionID, scope.RouteID},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.key)
	if err != nil {
		panic("auth: sign ticket JWT: " + err.Error())
	}
	return token, expiresAt
}

// Redeem validates a token against the request's scope and consumes it
// (single-use). Any mismatch, expiry, or reuse returns ErrTicketInvalid.
func (s *TicketStore) Redeem(tokenString string, want TicketScope) error {
	claims := &ticketClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.key, nil
	}, jwt.WithIssuer(ticketIssuer), jwt.WithAudience(want.ConnectionID, want.RouteID))
	if err != nil ||
		!token.Valid ||
		claims.Purpose != ticketPurpose ||
		claims.Subject != want.UserID ||
		claims.ID == "" ||
		claims.ExpiresAt == nil ||
		!sameAudience(claims.Audience, []string{want.ConnectionID, want.RouteID}) ||
		!sameParams(claims.Params, want.Params) {
		return ErrTicketInvalid
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return ErrTicketInvalid
	}
	lease, err := s.leases.Claim(context.Background(), "ticket:"+claims.ID, s.instance, livelease.ClaimOptions{Mode: livelease.ClaimExclusive, TTL: ttl})
	if err != nil {
		return ErrTicketInvalid
	}
	time.AfterFunc(ttl, func() { _ = lease.Release(context.Background()) })
	return nil
}

func cloneParams(params map[string]string) map[string]string {
	if params == nil {
		return nil
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		out[k] = v
	}
	return out
}

func sameParams(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func sameAudience(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, v := range got {
		seen[v] = true
	}
	for _, v := range want {
		if !seen[v] {
			return false
		}
	}
	return true
}

// CheckWSOrigin reports whether the request's Origin header is same-site with
// the Host (or matches an explicit allowlist). A missing Origin is rejected for
// cross-origin safety on the WS upgrade path.
func CheckWSOrigin(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, a := range allowed {
		if u.Host == a {
			return true
		}
	}
	return false
}
