package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/store"
)

// DefaultProxyImage is the container image install artifacts default to.
const DefaultProxyImage = "shellcn-proxy:latest"

// DefaultEnrollmentTTL is how long an unused enrollment token stays valid.
const DefaultEnrollmentTTL = 15 * time.Minute

var (
	// ErrNoAgentSupport is returned when a connection's plugin declares no agent.
	ErrNoAgentSupport = errors.New("service: plugin does not support agent transport")
	// ErrEnrollmentInvalid is returned for an unknown/expired/used token.
	ErrEnrollmentInvalid = errors.New("service: invalid enrollment token")
)

// InstallArtifact is a rendered launch recipe shown to the user.
type InstallArtifact struct {
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Command string `json:"command,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Enrollment is the response to creating an enrollment.
type Enrollment struct {
	EnrollmentID string            `json:"enrollmentId"`
	ExpiresAt    time.Time         `json:"expiresAt"`
	Artifacts    []InstallArtifact `json:"artifacts"`
}

// AgentState is the polled status the UI shows in the enroll panel.
type AgentState struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// EnrollmentService issues connection-scoped agent enrollment tokens and tracks
// the lifecycle of the resulting tunnels. The raw token only ever appears in a
// rendered install artifact; the store keeps a hash.
type EnrollmentService struct {
	store   store.EnrollmentStore
	conns   store.ConnectionStore
	plugins *plugin.Registry
	ttl     time.Duration
	now     func() time.Time
}

// NewEnrollmentService wires the dependencies.
func NewEnrollmentService(s store.EnrollmentStore, conns store.ConnectionStore, plugins *plugin.Registry) *EnrollmentService {
	return &EnrollmentService{store: s, conns: conns, plugins: plugins, ttl: DefaultEnrollmentTTL, now: time.Now}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Create issues a token for connectionID and renders the plugin's install
// artifacts (with the raw token + the gateway connect URL interpolated).
func (s *EnrollmentService) Create(ctx context.Context, connectionID, connectURL string) (Enrollment, error) {
	conn, err := s.conns.Get(ctx, connectionID)
	if err != nil {
		return Enrollment{}, err
	}
	m, ok := s.plugins.Manifest(conn.Protocol)
	if !ok || m.Agent == nil || !m.SupportsTransport(plugin.TransportAgent) {
		return Enrollment{}, ErrNoAgentSupport
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Enrollment{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	now := s.now()
	enr := &models.AgentEnrollment{
		ID:           uuid.NewString(),
		ConnectionID: connectionID,
		TokenHash:    hashToken(token),
		Status:       models.EnrollmentPending,
		ExpiresAt:    now.Add(s.ttl),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.Create(ctx, enr); err != nil {
		return Enrollment{}, err
	}

	artifacts, err := renderArtifacts(m.Agent.Install, connectURL, token)
	if err != nil {
		return Enrollment{}, err
	}
	return Enrollment{EnrollmentID: enr.ID, ExpiresAt: enr.ExpiresAt, Artifacts: artifacts}, nil
}

func renderArtifacts(specs []plugin.InstallArtifact, connectURL, token string) ([]InstallArtifact, error) {
	data := struct{ ConnectURL, Token, Image string }{ConnectURL: connectURL, Token: token, Image: DefaultProxyImage}
	out := make([]InstallArtifact, 0, len(specs))
	for _, spec := range specs {
		tmpl, err := template.New(spec.Kind).Parse(spec.Template)
		if err != nil {
			return nil, fmt.Errorf("render artifact %q: %w", spec.Kind, err)
		}
		var sb strings.Builder
		if err := tmpl.Execute(&sb, data); err != nil {
			return nil, fmt.Errorf("render artifact %q: %w", spec.Kind, err)
		}
		out = append(out, InstallArtifact{Label: spec.Label, Kind: spec.Kind, Command: sb.String()})
	}
	return out, nil
}

// Redeem validates an agent-presented token and returns the connection it binds
// to plus the target the agent should proxy. Single-use: a pending enrollment
// flips to online.
func (s *EnrollmentService) Redeem(ctx context.Context, token string) (connectionID string, proxy plugin.ProxyTarget, err error) {
	enr, err := s.store.GetByTokenHash(ctx, hashToken(token))
	if err != nil {
		return "", plugin.ProxyTarget{}, ErrEnrollmentInvalid
	}
	conn, err := s.conns.Get(ctx, enr.ConnectionID)
	if err != nil {
		return "", plugin.ProxyTarget{}, ErrEnrollmentInvalid
	}
	m, ok := s.plugins.Manifest(conn.Protocol)
	if !ok || m.Agent == nil {
		return "", plugin.ProxyTarget{}, ErrNoAgentSupport
	}
	// Atomic single-use gate: only the caller that flips pending→online wins, so
	// two agents racing the same token cannot both enroll.
	consumed, err := s.store.Consume(ctx, enr.ID, s.now())
	if err != nil {
		return "", plugin.ProxyTarget{}, err
	}
	if !consumed {
		return "", plugin.ProxyTarget{}, ErrEnrollmentInvalid
	}
	return conn.ID, m.Agent.Proxy, nil
}

// MarkOffline flips a connection's online enrollment to offline (tunnel closed).
func (s *EnrollmentService) MarkOffline(ctx context.Context, connectionID string) {
	enrollments, err := s.store.ListByConnection(ctx, connectionID)
	if err != nil {
		return
	}
	for _, e := range enrollments {
		if e.Status == models.EnrollmentOnline {
			_ = s.store.UpdateStatus(ctx, e.ID, models.EnrollmentOffline)
		}
	}
}

// State returns the latest agent status for a connection (for the enroll panel).
func (s *EnrollmentService) State(ctx context.Context, connectionID string) AgentState {
	enrollments, err := s.store.ListByConnection(ctx, connectionID)
	if err != nil || len(enrollments) == 0 {
		return AgentState{Status: string(models.EnrollmentOffline), Message: "No agent enrolled yet."}
	}
	latest := enrollments[0]
	for _, e := range enrollments[1:] {
		if e.CreatedAt.After(latest.CreatedAt) {
			latest = e
		}
	}
	st := AgentState{Status: string(latest.Status)}
	switch latest.Status {
	case models.EnrollmentPending:
		st.Message = "Waiting for the agent to dial back."
	case models.EnrollmentOnline:
		st.Message = "Agent connected."
	}
	return st
}
