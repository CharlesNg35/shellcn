package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/store"
)

// DefaultProxyImage is the container image install artifacts default to.
const DefaultProxyImage = "ghcr.io/charlesng35/shellcn-agent:latest"

// DefaultEnrollmentTTL is how long an unused enrollment token stays valid.
const DefaultEnrollmentTTL = 15 * time.Minute

var (
	// ErrNoAgentSupport is returned when a connection's plugin declares no agent.
	ErrNoAgentSupport = errors.New("service: plugin does not support agent transport")
	// ErrEnrollmentInvalid is returned for an unknown, expired, or revoked token.
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

// ArtifactURLFunc mints a single-use signed fetch URL for a URL-delivered
// artifact (supplied by the server, which owns ticket minting and the host).
type ArtifactURLFunc func(enrollmentID, kind string) (string, error)

// Create issues an enrollment and renders the plugin's install artifacts. URL
// artifacts defer token minting to the fetch (the record gets a non-redeemable
// placeholder hash) so the token only ever lands in the fetched body.
func (s *EnrollmentService) Create(ctx context.Context, connectionID, connectURL string, artifactURL ArtifactURLFunc) (Enrollment, error) {
	conn, err := s.conns.Get(ctx, connectionID)
	if err != nil {
		return Enrollment{}, err
	}
	m, ok := s.plugins.Manifest(conn.Protocol)
	if !ok || m.Agent == nil || !m.SupportsTransport(plugin.TransportAgent) {
		return Enrollment{}, ErrNoAgentSupport
	}
	urlMode, err := deliveryMode(m.Agent.Install)
	if err != nil {
		return Enrollment{}, err
	}

	now := s.now()
	enr := &models.AgentEnrollment{
		ID:           uuid.NewString(),
		ConnectionID: connectionID,
		Status:       models.EnrollmentPending,
		ExpiresAt:    now.Add(s.ttl),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	var token string
	if urlMode {
		// No real token yet — store a unique, non-redeemable placeholder so the
		// unique index holds; the real token is minted when the body is fetched.
		placeholder, err := randomToken()
		if err != nil {
			return Enrollment{}, err
		}
		enr.TokenHash = hashToken("placeholder:" + placeholder)
	} else {
		if token, err = randomToken(); err != nil {
			return Enrollment{}, err
		}
		enr.TokenHash = hashToken(token)
	}
	if err := s.store.Create(ctx, enr); err != nil {
		return Enrollment{}, err
	}

	artifacts, err := s.renderArtifacts(m.Agent.Install, connectURL, token, connectionSlug(conn), enr.ID, artifactURL)
	if err != nil {
		return Enrollment{}, err
	}
	return Enrollment{EnrollmentID: enr.ID, ExpiresAt: enr.ExpiresAt, Artifacts: artifacts}, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// deliveryMode reports whether the artifact set is URL-delivered. Mixing inline
// and URL delivery in one enrollment is unsupported: a URL set mints its token
// lazily and so has no token to inline.
func deliveryMode(specs []plugin.InstallArtifact) (urlMode bool, err error) {
	var sawURL, sawInline bool
	for _, a := range specs {
		if a.Delivery == plugin.DeliveryURL {
			sawURL = true
		} else {
			sawInline = true
		}
	}
	if sawURL && sawInline {
		return false, fmt.Errorf("%w: install artifacts mix inline and url delivery", ErrNoAgentSupport)
	}
	return sawURL, nil
}

func (s *EnrollmentService) renderArtifacts(specs []plugin.InstallArtifact, connectURL, token, slug, enrollmentID string, artifactURL ArtifactURLFunc) ([]InstallArtifact, error) {
	out := make([]InstallArtifact, 0, len(specs))
	for _, spec := range specs {
		data := artifactRenderData(connectURL, token, slug, spec.ConnectURL)
		if spec.Delivery == plugin.DeliveryURL {
			if artifactURL == nil {
				return nil, fmt.Errorf("%w: url artifact %q has no URL minter", ErrNoAgentSupport, spec.Kind)
			}
			u, err := artifactURL(enrollmentID, spec.Kind)
			if err != nil {
				return nil, err
			}
			data.ArtifactURL = u
			cmd, err := renderTemplate(spec.Kind, spec.Template, data)
			if err != nil {
				return nil, err
			}
			out = append(out, InstallArtifact{Label: spec.Label, Kind: spec.Kind, Command: cmd, URL: u})
			continue
		}
		cmd, err := renderTemplate(spec.Kind, spec.Template, data)
		if err != nil {
			return nil, err
		}
		out = append(out, InstallArtifact{Label: spec.Label, Kind: spec.Kind, Command: cmd})
	}
	return out, nil
}

func renderTemplate(name, tmpl string, data artifactData) (string, error) {
	t, err := template.New(name).Funcs(template.FuncMap{"shellquote": shellQuote}).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("render artifact %q: %w", name, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("render artifact %q: %w", name, err)
	}
	return sb.String(), nil
}

// RenderArtifactContent mints the real enrollment token and renders a
// URL-delivered artifact's body. The caller's single-use ticket bounds it to one
// fetch per enrollment.
func (s *EnrollmentService) RenderArtifactContent(ctx context.Context, connectionID, enrollmentID, kind, connectURL string) (string, error) {
	enr, err := s.store.Get(ctx, enrollmentID)
	if err != nil || enr.ConnectionID != connectionID {
		return "", ErrEnrollmentInvalid
	}
	conn, err := s.conns.Get(ctx, enr.ConnectionID)
	if err != nil {
		return "", ErrEnrollmentInvalid
	}
	m, ok := s.plugins.Manifest(conn.Protocol)
	if !ok || m.Agent == nil {
		return "", ErrNoAgentSupport
	}
	var spec *plugin.InstallArtifact
	for i := range m.Agent.Install {
		if m.Agent.Install[i].Kind == kind && m.Agent.Install[i].Delivery == plugin.DeliveryURL {
			spec = &m.Agent.Install[i]
			break
		}
	}
	if spec == nil {
		return "", fmt.Errorf("%w: no url artifact %q", ErrEnrollmentInvalid, kind)
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	now := s.now()
	if err := s.store.UpdateToken(ctx, enr.ID, hashToken(token), now.Add(s.ttl)); err != nil {
		return "", err
	}
	data := artifactRenderData(connectURL, token, connectionSlug(conn), spec.ConnectURL)
	return renderTemplate(spec.Kind, spec.Content, data)
}

type artifactData struct {
	GatewayConnectURL string
	ConnectURL        string
	ArtifactURL       string
	Token             string
	// Slug is a unique, DNS-1123-safe identifier for the connection (slugified
	// name + short id), so a plugin can name per-connection target-side resources
	// without two connections colliding.
	Slug                  string
	Image                 string
	Insecure              bool
	LocalhostHost         string
	LocalhostHostRequired bool
}

func artifactRenderData(connectURL, token, slug string, target plugin.ArtifactConnectURL) artifactData {
	renderedURL, localhostRequired := rewriteConnectURL(connectURL, target)
	return artifactData{
		GatewayConnectURL:     connectURL,
		ConnectURL:            renderedURL,
		Token:                 token,
		Slug:                  slug,
		Image:                 DefaultProxyImage,
		Insecure:              insecureConnectURL(connectURL),
		LocalhostHost:         target.LocalhostHost,
		LocalhostHostRequired: localhostRequired,
	}
}

func insecureConnectURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "ws"
}

func rewriteConnectURL(raw string, target plugin.ArtifactConnectURL) (string, bool) {
	if strings.TrimSpace(target.LocalhostHost) == "" {
		return raw, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(target.LocalhostHost, port)
		} else {
			u.Host = target.LocalhostHost
		}
		return u.String(), true
	default:
		return raw, false
	}
}

// connectionSlug is a unique, DNS-1123-safe identifier for a connection: the
// slugified name plus a short id fragment. The id keeps it unique even when two
// connections share a name, so per-connection target-side resources don't clash.
func connectionSlug(conn models.Connection) string {
	slug := slugify(conn.Name)
	short := strings.ReplaceAll(conn.ID, "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	if slug == "" {
		slug = "shellcn-agent"
	}
	if short != "" {
		slug += "-" + short
	}
	if len(slug) > 63 {
		slug = slug[:63]
	}
	return strings.Trim(slug, "-")
}

// slugify folds accents and lowercases s into a DNS-1123 label fragment
// (a-z, 0-9, '-'), collapsing and trimming separators.
func slugify(s string) string {
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		s,
	)
	if err != nil {
		folded = s
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(folded) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// Redeem validates an agent-presented token and returns the connection it binds
// to plus the target the agent should proxy. An unused pending token must still
// be within its install window; an already-enrolled agent may reconnect with the
// same token until that enrollment is revoked.
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
	case models.EnrollmentOffline:
		st.Message = "Agent disconnected."
	}
	return st
}
