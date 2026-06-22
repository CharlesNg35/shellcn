package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/hashicorp/yamux"

	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/email"
	"github.com/charlesng35/shellcn/internal/livelease"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/server"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/transport"
	shellssh "github.com/charlesng35/shellcn/plugins/ssh"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// --- test plugins -----------------------------------------------------------

type fakeSess struct{}

func (fakeSess) HealthCheck(context.Context) error { return nil }
func (fakeSess) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}
func (fakeSess) Close() error { return nil }
func (fakeSess) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("proxied:" + r.URL.Path))
}

type testPlugin struct{}

var schemaOnlyCalls atomic.Int32

func (testPlugin) Manifest() plugin.Manifest {
	directOnly := plugin.Condition{AllOf: []plugin.Rule{{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)}}}
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "tester", Version: "0", Title: "Tester", Category: plugin.CategoryOther,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent},
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, VisibleWhen: &directOnly},
			{Key: "read_only", Label: "Read-only", Type: plugin.FieldToggle, Default: true},
			{Key: "direct_secret", Label: "Direct secret", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &directOnly},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true},
			{
				Key: "credential_id", Label: "Credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindDBPassword},
			},
			{
				Key: "api_credential", Label: "API Credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindAPIToken},
			},
		}}}},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentTCP, Address: "127.0.0.1:1", Risk: plugin.RiskPrivileged},
			Install: []plugin.InstallArtifact{
				{Label: "Docker", Kind: "docker", Template: "run {{.ConnectURL}} {{.Token}}"},
			},
		},
		Tabs: []plugin.Panel{{Key: "items", Label: "Items", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "tester.list"}}},
		Streams: []plugin.Stream{
			{ID: "tester.ws", Kind: plugin.StreamTerminal, RouteID: "tester.ws"},
			{ID: "tester.desk", Kind: plugin.StreamDesktop, RouteID: "tester.desk"},
		},
		Recording: []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"tester.ws"}, Authoritative: true},
			{Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas}, StreamIDs: []string{"tester.desk"}},
		},
	}
}

func (testPlugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "tester.list", Method: plugin.MethodGet, Permission: "tester.read", Risk: plugin.RiskSafe, AuditEvent: "tester.list",
			Handle: func(*plugin.RequestContext) (any, error) { return plugin.Page[string]{Items: []string{"a", "b"}}, nil },
		},
		{
			ID: "tester.unauth", Method: plugin.MethodGet, Permission: "tester.read", Risk: plugin.RiskSafe, AuditEvent: "tester.unauth",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, plugin.ErrUnauthorized },
		},
		{
			ID: "tester.echoparam", Method: plugin.MethodGet, Permission: "tester.read", Risk: plugin.RiskSafe, AuditEvent: "tester.echoparam",
			Path:   "/echo/{name}",
			Handle: func(rc *plugin.RequestContext) (any, error) { return map[string]string{"name": rc.Param("name")}, nil },
		},
		{
			ID: "tester.danger", Method: plugin.MethodDelete, Permission: "tester.delete", Risk: plugin.RiskDestructive, AuditEvent: "tester.danger",
			Handle: func(*plugin.RequestContext) (any, error) { return map[string]bool{"ok": true}, nil },
		},
		{
			ID: "tester.input", Method: plugin.MethodPost, Permission: "tester.write", Risk: plugin.RiskWrite, AuditEvent: "tester.input",
			Handle: func(rc *plugin.RequestContext) (any, error) {
				var body struct {
					Name string `json:"name" validate:"required"`
				}
				if err := rc.Bind(&body); err != nil {
					return nil, err
				}
				return map[string]string{"name": body.Name}, nil
			},
		},
		{
			ID: "tester.schema", Method: plugin.MethodPost, Permission: "tester.write", Risk: plugin.RiskWrite, AuditEvent: "tester.schema",
			Input: &plugin.Schema{Groups: []plugin.Group{{Name: "Input", Fields: []plugin.Field{
				{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
			}}}},
			Handle: func(*plugin.RequestContext) (any, error) {
				schemaOnlyCalls.Add(1)
				return map[string]bool{"ok": true}, nil
			},
		},
		{
			ID: "tester.upload", Method: plugin.MethodPost, Permission: "tester.write", Risk: plugin.RiskWrite, AuditEvent: "tester.upload",
			Input: &plugin.Schema{Groups: []plugin.Group{{Name: "Upload", Fields: []plugin.Field{
				{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
				{Key: "files", Label: "Files", Type: plugin.FieldFile, Required: true},
			}}}},
			Handle: func(rc *plugin.RequestContext) (any, error) {
				var body struct {
					Name string `json:"name" validate:"required"`
				}
				if err := rc.Bind(&body); err != nil {
					return nil, err
				}
				files := rc.Uploads("files")
				if len(files) == 0 {
					return nil, plugin.ErrInvalidInput
				}
				return map[string]any{"name": body.Name, "filename": files[0].Filename, "size": files[0].Size}, nil
			},
		},
		{
			ID: "tester.ws", Method: plugin.MethodWS, Permission: "tester.read", Risk: plugin.RiskSafe, AuditEvent: "tester.ws",
			Stream: func(_ *plugin.RequestContext, c plugin.ClientStream) error {
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				_, _ = c.Write(buf[:n])
				return nil
			},
		},
		{
			ID: "tester.desk", Method: plugin.MethodWS, Permission: "tester.read", Risk: plugin.RiskPrivileged, AuditEvent: "tester.desk",
			Stream: func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil },
		},
	}
}

func (testPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return fakeSess{}, nil
}

type internalPlugin struct{}

func (internalPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "internal",
		Version:             "0",
		Title:               "Internal Test",
		Category:            plugin.CategoryOther,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{
			{Key: "items", Label: "Items", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "internal.list"}},
			{Key: "echo", Label: "Echo", Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "internal.echo", Method: plugin.MethodWS}},
		},
		Streams: []plugin.Stream{{ID: "internal.echo", Kind: plugin.StreamTerminal, RouteID: "internal.echo"}},
	}
}

func (internalPlugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "internal.list", Method: plugin.MethodGet, Permission: "internal.read", Risk: plugin.RiskSafe, AuditEvent: "internal.list",
			Handle: func(*plugin.RequestContext) (any, error) {
				return plugin.Page[string]{Items: []string{"alpha", "bravo"}}, nil
			},
		},
		{
			ID: "internal.echo", Method: plugin.MethodWS, Permission: "internal.read", Risk: plugin.RiskSafe, AuditEvent: "internal.echo",
			Stream: func(_ *plugin.RequestContext, c plugin.ClientStream) error {
				if _, err := c.Write([]byte("internal echo ready\n")); err != nil {
					return err
				}
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				_, _ = c.Write(buf[:n])
				return nil
			},
		},
	}
}

func (internalPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return fakeSess{}, nil
}

type agentOnlyPlugin struct{}

func (agentOnlyPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "agentonly", Version: "0", Title: "Agent Only",
		Category:            plugin.CategoryOther,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy:   plugin.ProxyTarget{Mode: plugin.AgentTCP, Address: "127.0.0.1:1", Risk: plugin.RiskSafe},
			Install: []plugin.InstallArtifact{{Label: "Shell", Kind: "shell", Template: "run {{.Token}}"}},
		},
		Tabs: []plugin.Panel{{Key: "items", Label: "Items", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "agentonly.list"}}},
	}
}

func (agentOnlyPlugin) Routes() []plugin.Route {
	return []plugin.Route{{
		ID: "agentonly.list", Method: plugin.MethodGet, Permission: "agentonly.read", Risk: plugin.RiskSafe, AuditEvent: "agentonly.list",
		Handle: func(*plugin.RequestContext) (any, error) { return plugin.Page[string]{}, nil },
	}}
}

func (agentOnlyPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return fakeSess{}, nil
}

// boomPlugin fails to Connect, so any route on it resolves but the session is unavailable.
type boomPlugin struct{}

func (boomPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "boom", Version: "0", Title: "Boom",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs, SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
}

func (boomPlugin) Routes() []plugin.Route {
	return []plugin.Route{{
		ID: "boom.list", Method: plugin.MethodGet, Permission: "boom.read", Risk: plugin.RiskSafe, AuditEvent: "boom.list",
		Handle: func(*plugin.RequestContext) (any, error) { return "unreached", nil },
	}}
}

func (boomPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, plugin.ErrUnavailable
}

// --- harness ----------------------------------------------------------------

type harness struct {
	ts             *httptest.Server
	srv            *server.Server
	store          *store.Store
	tunnels        *transport.Registry
	pluginSessions *session.Manager
	sessionMgr     *auth.SessionManager
	sessions       map[string]auth.Session // userID → platform session
}

func newHarness(t *testing.T, opts ...func(*server.Deps)) *harness {
	t.Helper()
	st := store.NewMemory()
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)

	reg := pluginregistry.New()
	reg.MustRegister(testPlugin{})
	reg.MustRegister(boomPlugin{})
	reg.MustRegister(internalPlugin{})
	reg.MustRegister(agentOnlyPlugin{})
	reg.MustRegister(shellssh.New())
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	pol, err := policy.New()
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	instance := livelease.NewInstanceRef("test-instance", "http://test-instance")
	leases := livelease.NewStoreLeaseRegistry(st.LiveStateLeases)
	sessMgr := session.New(session.Options{LeaseRegistry: leases, Instance: instance})
	t.Cleanup(sessMgr.Shutdown)
	tunnels := transport.NewRegistry(transport.WithLeaseRegistry(leases, instance))
	connector := service.NewConnector(reg, creds, vault, tunnels)
	connections := service.NewConnectionService(st.Connections, reg, creds, vault)
	recBlobs, err := recording.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	recEngine := recording.NewEngine(recording.Options{Store: st.Recordings, Blobs: recBlobs})
	recEngine.Register(plugin.FormatAsciicastV2, recording.NewAsciicastRecorder)
	recordings := service.NewRecordingService(st.Recordings, recBlobs)
	authMgr := auth.NewSessionManager(time.Hour)
	ticketKey := []byte("0123456789abcdef0123456789abcdef")
	enrollments := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)
	users := service.NewUserService(st.Users)
	twoFactor := service.NewTwoFactorService(st.Users, vault, "ShellCN")
	invitations := service.NewInvitationService(st.Invitations, users, email.New(email.SMTP{}))

	deps := server.Deps{
		Plugins: reg, Store: st, Sessions: sessMgr,
		Auth: auth.NewLocalAuthenticator(st.Users), SessionMgr: authMgr,
		Tickets: auth.NewTicketStore(auth.TicketStoreOptions{
			TTL:        time.Minute,
			SigningKey: ticketKey,
			Leases:     leases,
			Instance:   instance,
		}),
		Policy:    pol,
		Connector: connector, Connections: connections, Credentials: creds, Audit: audit.NewWriter(st.Audit),
		Enrollments: enrollments, Tunnels: tunnels, Leases: leases, Instance: instance, Protocols: service.NewProtocolService(st.ProtocolSettings),
		Users: users, TwoFactor: twoFactor, Invitations: invitations,
		Recording: recEngine, Recordings: recordings,
		AI: aiconfig.New(st.AIProviders, vault, config.AIConfig{
			Kind: "openai", Name: "Shared", APIKey: "sk-global-secret", Model: "gpt-4o",
		}),
		ModelRegistry: modelreg.New(modelreg.WithoutRegistryFetch()),
	}
	for _, o := range opts {
		o(&deps)
	}
	srv := server.New(deps)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	h := &harness{ts: ts, srv: srv, store: st, tunnels: tunnels, pluginSessions: sessMgr, sessionMgr: authMgr, sessions: map[string]auth.Session{}}

	ctx := context.Background()
	for _, u := range []struct {
		id   string
		role models.Role
	}{{"admin", models.RoleAdmin}, {"op", models.RoleOperator}, {"op2", models.RoleOperator}, {"viewer", models.RoleViewer}} {
		_ = st.Users.Create(ctx, &models.User{ID: u.id, Username: u.id, Roles: []models.Role{u.role}}, "")
		h.sessions[u.id] = authMgr.Create(u.id)
	}
	// Connections: op owns a tester + a boom connection; viewer owns a tester.
	_ = st.Connections.Create(ctx, &models.Connection{ID: "c-op", Name: "op", Protocol: "tester", OwnerID: "op", Transport: "direct"})
	_ = st.Connections.Create(ctx, &models.Connection{ID: "c-boom", Name: "boom", Protocol: "boom", OwnerID: "op", Transport: "direct"})
	_ = st.Connections.Create(ctx, &models.Connection{ID: "c-view", Name: "v", Protocol: "tester", OwnerID: "viewer", Transport: "direct"})
	_ = st.Connections.Create(ctx, &models.Connection{ID: "c-internal", Name: "internal", Protocol: "internal", OwnerID: "op", Transport: "direct"})
	return h
}

// apiResp is a fully-consumed HTTP response (body read + closed).
type apiResp struct {
	Status int
	Body   []byte
}

func (h *harness) do(t *testing.T, method, path, userID string, body io.Reader) apiResp {
	t.Helper()
	req, err := http.NewRequest(method, h.ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if userID != "" {
		sess := h.sessions[userID]
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.ID})
		if method != http.MethodGet {
			req.Header.Set(auth.CSRFHeader, sess.CSRFToken)
		}
	}
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return apiResp{Status: resp.StatusCode, Body: b}
}

func (h *harness) doReq(t *testing.T, req *http.Request, userID string) apiResp {
	t.Helper()
	if userID != "" {
		sess := h.sessions[userID]
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.ID})
		if req.Method != http.MethodGet {
			req.Header.Set(auth.CSRFHeader, sess.CSRFToken)
		}
	}
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return apiResp{Status: resp.StatusCode, Body: b}
}

func TestLeaseProxyForwardsRemoteOwner(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		if got := r.Header.Get("X-ShellCN-Lease-Proxy"); got != "test-instance" {
			t.Fatalf("lease proxy header = %q, want test-instance", got)
		}
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	t.Cleanup(target.Close)

	leases := livelease.NewStoreLeaseRegistry(h.store.LiveStateLeases)
	lease, err := leases.Claim(context.Background(), livelease.SessionLeaseKey("c-op", "op"), livelease.NewInstanceRef("remote-instance", target.URL), livelease.ClaimOptions{
		Mode: livelease.ClaimExclusive,
		TTL:  time.Minute,
	})
	if err != nil {
		t.Fatalf("claim remote session lease: %v", err)
	}
	t.Cleanup(func() { _ = lease.Release(context.Background()) })

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("session status via proxy: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if got, want := string(resp.Body), "/api/connections/c-op/session"; got != want {
		t.Fatalf("proxied path = %q, want %q", got, want)
	}
}

func TestLeaseProxyFallsBackAndPromotesReachableOwnerURL(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	t.Cleanup(target.Close)

	leases := livelease.NewStoreLeaseRegistry(h.store.LiveStateLeases)
	lease, err := leases.Claim(context.Background(), livelease.SessionLeaseKey("c-op", "op"), livelease.NewInstanceRef("remote-instance", "http://127.0.0.1:1", target.URL), livelease.ClaimOptions{
		Mode: livelease.ClaimExclusive,
		TTL:  time.Minute,
	})
	if err != nil {
		t.Fatalf("claim remote session lease: %v", err)
	}
	t.Cleanup(func() { _ = lease.Release(context.Background()) })

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("session status via fallback proxy: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	ref, ok, err := leases.Get(context.Background(), livelease.SessionLeaseKey("c-op", "op"))
	if err != nil || !ok {
		t.Fatalf("get ref: ok=%v err=%v", ok, err)
	}
	if ref.Instance.PreferredInternalURL() != target.URL {
		t.Fatalf("preferred URL = %q, want %q", ref.Instance.PreferredInternalURL(), target.URL)
	}
}

func TestLeaseProxyLoopGuard(t *testing.T) {
	h := newHarness(t)
	leases := livelease.NewStoreLeaseRegistry(h.store.LiveStateLeases)
	lease, err := leases.Claim(context.Background(), livelease.SessionLeaseKey("c-op", "op"), livelease.NewInstanceRef("remote-instance", h.ts.URL), livelease.ClaimOptions{
		Mode: livelease.ClaimExclusive,
		TTL:  time.Minute,
	})
	if err != nil {
		t.Fatalf("claim remote session lease: %v", err)
	}
	t.Cleanup(func() { _ = lease.Release(context.Background()) })

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("loop guard: want 503, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestWrapperOrder(t *testing.T) {
	h := newHarness(t)

	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "", nil); resp.Status != http.StatusUnauthorized {
		t.Errorf("unauthenticated: want 401, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/ghost", "op", nil); resp.Status != http.StatusNotFound {
		t.Errorf("unknown route: want 404, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodDelete, "/api/connections/c-view/x/tester.danger", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("viewer destructive: want 403, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodGet, "/api/connections/c-boom/x/boom.list", "op", nil); resp.Status != http.StatusServiceUnavailable {
		t.Errorf("connect failure: want 503, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/x/tester.input", "op", strings.NewReader(`{}`)); resp.Status != http.StatusBadRequest {
		t.Errorf("bad input: want 400, got %d", resp.Status)
	}

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("happy: want 200, got %d", resp.Status)
	}
	rows, _ := h.store.Audit.List(context.Background(), store.AuditFilter{ConnectionID: "c-op"})
	var found bool
	for _, r := range rows {
		if r.Event == "tester.list" && r.Result == models.AuditAllowed {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an allowed audit row for tester.list, got %+v", rows)
	}
}

func TestConnectionProxyLazilyAcquiresSession(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/proxy/services/default/app/80/en/login", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("proxy: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if got, want := string(resp.Body), "proxied:/services/default/app/80/en/login"; got != want {
		t.Fatalf("proxy body = %q, want %q", got, want)
	}

	snap, ok := h.pluginSessions.Status(session.Key{ConnectionID: "c-op", ActorScope: "op"})
	if !ok {
		t.Fatal("proxy request should create a live session")
	}
	if snap.State != session.StateConnected {
		t.Fatalf("proxy session state = %s, want %s", snap.State, session.StateConnected)
	}
}

// A proxied third-party app cannot carry our CSRF token, so a state-changing
// request through the proxy must not be rejected; the cookie + SameSite + route
// authz still guard it.
func TestConnectionProxyExemptFromCSRF(t *testing.T) {
	h := newHarness(t)
	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/connections/c-op/proxy/services/default/app/8080/api/items", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: h.sessions["op"].ID})
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy POST without CSRF: want 200, got %d (%s)", resp.StatusCode, b)
	}
}

func TestParamResolution(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.echoparam?p.name=resolved", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.Status)
	}
	b := resp.Body
	if !strings.Contains(string(b), "resolved") {
		t.Errorf("p.name param did not resolve into rc.Param: %s", b)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.echoparam", "op", nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("missing declared param: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.echoparam?p.name=x&p.extra=y", "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("scoped extra param: want 200, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestMultipartRouteBinding(t *testing.T) {
	h := newHarness(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("name", "release"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("files", "release.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("artifact")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/connections/c-op/x/tester.upload", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp := h.doReq(t, req, "op")
	if resp.Status != http.StatusOK {
		t.Fatalf("upload: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	got := string(resp.Body)
	for _, want := range []string{`"name":"release"`, `"filename":"release.txt"`, `"size":8`} {
		if !strings.Contains(got, want) {
			t.Errorf("upload response missing %s: %s", want, got)
		}
	}
}

func TestMultipartRejectedWithoutFileInputSchema(t *testing.T) {
	h := newHarness(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("name", "release")
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/connections/c-op/x/tester.input", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if resp := h.doReq(t, req, "op"); resp.Status != http.StatusBadRequest {
		t.Fatalf("multipart without file input schema: want 400, got %d", resp.Status)
	}
}

func TestWrapperValidatesDeclaredInputSchemaBeforeHandler(t *testing.T) {
	h := newHarness(t)
	schemaOnlyCalls.Store(0)

	resp := h.do(t, http.MethodPost, "/api/connections/c-op/x/tester.schema", "op", strings.NewReader(`{}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("schema invalid input: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	if got := schemaOnlyCalls.Load(); got != 0 {
		t.Fatalf("handler ran despite invalid declared input: calls=%d", got)
	}

	resp = h.do(t, http.MethodPost, "/api/connections/c-op/x/tester.schema", "op", strings.NewReader(`{"name":"release"}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("schema valid input: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if got := schemaOnlyCalls.Load(); got != 1 {
		t.Fatalf("handler call count = %d, want 1", got)
	}

	resp = h.do(t, http.MethodPost, "/api/connections/c-op/x/tester.schema", "op", strings.NewReader(`{"name":"release","extra":"nope"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("schema unknown field: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	if got := schemaOnlyCalls.Load(); got != 1 {
		t.Fatalf("handler ran despite unknown declared input: calls=%d", got)
	}
}

func TestDeniedAuthorizationIsAudited(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodDelete, "/api/connections/c-view/x/tester.danger", "viewer", nil)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("viewer destructive: want 403, got %d", resp.Status)
	}

	rows, err := h.store.Audit.List(context.Background(), store.AuditFilter{ConnectionID: "c-view"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.RouteID == "tester.danger" && row.Result == models.AuditDenied {
			return
		}
	}
	t.Fatalf("missing denied audit row for tester.danger: %+v", rows)
}

func TestAgentEnrollmentIsAudited(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/connections/c-op/agent/enrollments", "op", nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create enrollment: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "SHELLCN") && !strings.Contains(body, "agent/connect") {
		t.Fatalf("enrollment response missing install artifact: %s", resp.Body)
	}
	for _, want := range []string{`"enrollmentId":"`, `"expiresAt":"`, `"artifacts":[`, `"downloadUrl":"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("enrollment response missing client key %s: %s", want, body)
		}
	}
	for _, forbidden := range []string{`"EnrollmentID"`, `"ConnectionID"`, `"TokenHash"`, `"DownloadURL"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("enrollment response leaked server field %s: %s", forbidden, body)
		}
	}

	rows, err := h.store.Audit.List(context.Background(), store.AuditFilter{ConnectionID: "c-op"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.RouteID == "agent.enrollment.create" && row.Result == models.AuditAllowed && row.Risk == string(plugin.RiskPrivileged) {
			return
		}
	}
	t.Fatalf("missing agent enrollment audit row: %+v", rows)
}

func TestRejectedAgentConnectIsAudited(t *testing.T) {
	h := newHarness(t)
	c, resp, err := websocket.Dial(context.Background(), h.wsURL("/api/agent/connect"), nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial agent connect: %v", err)
	}
	defer func() { _ = c.CloseNow() }()

	if err := wsjson.Write(context.Background(), c, transport.AgentHello{Token: "not-a-token"}); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	var agentResp transport.AgentConnectResponse
	if err := wsjson.Read(context.Background(), c, &agentResp); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if agentResp.OK {
		t.Fatal("invalid token should be rejected")
	}

	rows, err := h.store.Audit.List(context.Background(), store.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.RouteID == "agent.connect" && row.Result == models.AuditDenied && row.Risk == string(plugin.RiskPrivileged) {
			return
		}
	}
	t.Fatalf("missing denied agent.connect audit row: %+v", rows)
}

func TestMalformedAgentConnectIsAudited(t *testing.T) {
	h := newHarness(t)
	c, resp, err := websocket.Dial(context.Background(), h.wsURL("/api/agent/connect"), nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial agent connect: %v", err)
	}
	_ = c.Close(websocket.StatusPolicyViolation, "no hello")

	waitForAudit(t, h, func(row models.AuditEntry) bool {
		return row.RouteID == "agent.connect" && row.Result == models.AuditDenied && row.Risk == string(plugin.RiskPrivileged)
	})
}

func TestReplacingAgentTunnelDoesNotMarkEnrollmentOffline(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/connections/c-op/agent/enrollments", "op", nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create enrollment: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	token := extractAgentToken(t, resp.Body)

	first := dialAgentTunnel(t, h, token, "first")
	defer first.close()
	second := dialAgentTunnel(t, h, token, "second")
	defer second.close()
	waitForCurrentTunnel(t, h, "c-op", "second")

	first.close()

	deadline := time.Now().Add(time.Second)
	for {
		resp = h.do(t, http.MethodGet, "/api/connections/c-op/agent/state", "op", nil)
		if resp.Status != http.StatusOK {
			t.Fatalf("agent state: want 200, got %d (%s)", resp.Status, resp.Body)
		}
		if strings.Contains(string(resp.Body), `"status":"online"`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale tunnel teardown marked active replacement offline: %s", resp.Body)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAgentEnrollmentUsesForwardedPublicURL(t *testing.T) {
	h := newHarness(t)
	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/connections/c-op/agent/enrollments", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "internal:8080"
	req.Header.Set("X-Forwarded-Host", "shellcn.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")

	resp := h.doReq(t, req, "op")
	if resp.Status != http.StatusCreated {
		t.Fatalf("create enrollment: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if !strings.Contains(string(resp.Body), "wss://shellcn.example.com/api/agent/connect") {
		t.Fatalf("enrollment response did not use forwarded public URL: %s", resp.Body)
	}
}

func TestAgentEnrollmentUsesAPIServerPortBehindLocalDevProxy(t *testing.T) {
	h := newHarness(t)
	serverURL, err := url.Parse(h.ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/connections/c-op/agent/enrollments", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "localhost:5173"

	resp := h.doReq(t, req, "op")
	if resp.Status != http.StatusCreated {
		t.Fatalf("create enrollment: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	want := "ws://localhost:" + serverURL.Port() + "/api/agent/connect"
	if !strings.Contains(string(resp.Body), want) {
		t.Fatalf("enrollment response did not use API server port %q: %s", want, resp.Body)
	}
}

func TestAgentStateTreatsTunnelRegistryAsAuthoritative(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	if err := h.store.Enrollments.Create(context.Background(), &models.AgentEnrollment{
		ID:           "stale-online",
		ConnectionID: "c-op",
		TokenHash:    "stale-online-token",
		Status:       models.EnrollmentOnline,
		ExpiresAt:    now.Add(time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/agent/state", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("state: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if !strings.Contains(string(resp.Body), `"status":"offline"`) {
		t.Fatalf("state should be offline without a live tunnel: %s", resp.Body)
	}
	if strings.Contains(string(resp.Body), `"Status"`) || strings.Contains(string(resp.Body), `"Message"`) {
		t.Fatalf("state response used server keys: %s", resp.Body)
	}
	enr, err := h.store.Enrollments.Get(context.Background(), "stale-online")
	if err != nil {
		t.Fatal(err)
	}
	if enr.Status != models.EnrollmentOffline {
		t.Fatalf("stale enrollment should be persisted offline, got %s", enr.Status)
	}
}

func TestAgentStateKeepsRemoteOwnerOnline(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	if err := h.store.Enrollments.Create(context.Background(), &models.AgentEnrollment{
		ID:           "remote-online",
		ConnectionID: "c-op",
		TokenHash:    "remote-online-token",
		Status:       models.EnrollmentOnline,
		ExpiresAt:    now.Add(time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}
	leases := livelease.NewStoreLeaseRegistry(h.store.LiveStateLeases)
	lease, err := leases.Claim(context.Background(), livelease.AgentLeaseKey("c-op"), livelease.NewInstanceRef("remote-instance", "http://remote"), livelease.ClaimOptions{
		Mode: livelease.ClaimReplace,
		TTL:  time.Minute,
	})
	if err != nil {
		t.Fatalf("claim remote lease: %v", err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/agent/state", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("state: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if !strings.Contains(string(resp.Body), `"status":"online"`) {
		t.Fatalf("remote lease should keep agent online: %s", resp.Body)
	}
	enr, err := h.store.Enrollments.Get(context.Background(), "remote-online")
	if err != nil {
		t.Fatal(err)
	}
	if enr.Status != models.EnrollmentOnline {
		t.Fatalf("remote-owned enrollment should stay online, got %s", enr.Status)
	}
}

func TestAdminCannotAccessOthersConnection(t *testing.T) {
	h := newHarness(t)
	// Admin is a user-management role, not a super-user: no implicit access to
	// another user's connection.
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "admin", nil); resp.Status != http.StatusForbidden {
		t.Errorf("admin on another's connection: want 403, got %d", resp.Status)
	}
}

func TestStrangerDeniedConnection(t *testing.T) {
	h := newHarness(t)
	// viewer is not ref/grantee of c-op → forbidden even for a safe route.
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("stranger on a connection: want 403, got %d", resp.Status)
	}
}

func TestProjectionEndpoints(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/api/plugins/tester", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("get plugin: want 200, got %d", resp.Status)
	}
	b := resp.Body
	if !strings.Contains(string(b), `"layout":"tabs"`) {
		t.Errorf("projection missing layout: %s", b)
	}

	resp = h.do(t, http.MethodGet, "/api/plugins", "op", nil)
	b = resp.Body
	if !strings.Contains(string(b), `"name":"tester"`) {
		t.Errorf("plugin list missing tester: %s", b)
	}
}

func (h *harness) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(h.ts.URL, "http") + path
}

func waitForAudit(t *testing.T, h *harness, match func(models.AuditEntry) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		rows, err := h.store.Audit.List(context.Background(), store.AuditFilter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if match(row) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing audit row: %+v", rows)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type testAgentTunnel struct {
	cancel context.CancelFunc
	ws     *websocket.Conn
	sess   *yamux.Session
}

func (t *testAgentTunnel) close() {
	if t.sess != nil {
		_ = t.sess.Close()
	}
	if t.ws != nil {
		_ = t.ws.CloseNow()
	}
	if t.cancel != nil {
		t.cancel()
	}
}

func dialAgentTunnel(t *testing.T, h *harness, token, label string) *testAgentTunnel {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c, resp, err := websocket.Dial(ctx, h.wsURL("/api/agent/connect"), nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		cancel()
		t.Fatalf("dial agent connect: %v", err)
	}
	if err := wsjson.Write(ctx, c, transport.AgentHello{Token: token, Forward: true}); err != nil {
		_ = c.CloseNow()
		cancel()
		t.Fatalf("write hello: %v", err)
	}
	var agentResp transport.AgentConnectResponse
	if err := wsjson.Read(ctx, c, &agentResp); err != nil {
		_ = c.CloseNow()
		cancel()
		t.Fatalf("read response: %v", err)
	}
	if !agentResp.OK {
		_ = c.CloseNow()
		cancel()
		t.Fatalf("agent rejected: %s", agentResp.Error)
	}
	nc := websocket.NetConn(ctx, c, websocket.MessageBinary)
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.LogOutput = io.Discard
	sess, err := yamux.Server(nc, cfg)
	if err != nil {
		_ = c.CloseNow()
		cancel()
		t.Fatalf("yamux server: %v", err)
	}
	go func() {
		for {
			stream, err := sess.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(stream, label)
			_ = stream.Close()
		}
	}()
	return &testAgentTunnel{cancel: cancel, ws: c, sess: sess}
}

func waitForCurrentTunnel(t *testing.T, h *harness, connectionID, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if dial, ok := h.tunnels.Dialer(connectionID); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			conn, err := dial(ctx, "tcp", "127.0.0.1:1")
			cancel()
			if err == nil {
				buf := make([]byte, len(want))
				_, readErr := io.ReadFull(conn, buf)
				_ = conn.Close()
				if readErr == nil && string(buf) == want {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("active tunnel did not become %q", want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func extractAgentToken(t *testing.T, body []byte) string {
	t.Helper()
	var resp struct {
		Artifacts []struct {
			Command string `json:"command"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode enrollment response: %v", err)
	}
	re := regexp.MustCompile(`\s([A-Za-z0-9_-]{43})(?:\s|$)`)
	for _, artifact := range resp.Artifacts {
		if match := re.FindStringSubmatch(artifact.Command); len(match) == 2 {
			return match[1]
		}
	}
	t.Fatalf("missing agent token in enrollment response: %s", body)
	return ""
}

func (h *harness) dialWS(t *testing.T, userID, path string) (*websocket.Conn, error) {
	t.Helper()
	sess := h.sessions[userID]
	hdr := http.Header{}
	hdr.Set("Cookie", auth.SessionCookieName+"="+sess.ID)
	hdr.Set("Origin", h.ts.URL) // same-site
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, h.wsURL(path), &websocket.DialOptions{HTTPHeader: hdr})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return c, err
}

func (h *harness) dialWSWithSubprotocol(t *testing.T, userID, path string, subprotocol string) (*websocket.Conn, error) {
	t.Helper()
	sess := h.sessions[userID]
	hdr := http.Header{}
	hdr.Set("Cookie", auth.SessionCookieName+"="+sess.ID)
	hdr.Set("Origin", h.ts.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, h.wsURL(path), &websocket.DialOptions{
		HTTPHeader:   hdr,
		Subprotocols: []string{subprotocol},
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return c, err
}

func (h *harness) mintTicket(t *testing.T, userID, connID, routeID string, params map[string]string) string {
	t.Helper()
	body := `{"routeId":"` + routeID + `","params":{`
	first := true
	for k, v := range params {
		if !first {
			body += ","
		}
		body += `"` + k + `":"` + v + `"`
		first = false
	}
	body += `}}`
	resp := h.do(t, http.MethodPost, "/api/connections/"+connID+"/tickets", userID, strings.NewReader(body))
	if resp.Status != http.StatusCreated {
		t.Fatalf("mint ticket: want 201, got %d", resp.Status)
	}
	b := resp.Body
	const k = `"ticket":"`
	i := strings.Index(string(b), k)
	if i < 0 {
		t.Fatalf("no ticket in %s", b)
	}
	rest := string(b)[i+len(k):]
	return rest[:strings.Index(rest, `"`)]
}

func TestAuthLoginFlow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("s3cret-pw")
	if err := h.store.Users.Create(ctx, &models.User{ID: "login1", Username: "loginuser", Roles: []models.Role{models.RoleViewer}}, hash); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Wrong password is rejected.
	if r := h.do(t, http.MethodPost, "/api/auth/login", "", strings.NewReader(`{"username":"loginuser","password":"nope"}`)); r.Status != http.StatusUnauthorized {
		t.Errorf("bad login: want 401, got %d", r.Status)
	}

	// Correct login returns a session cookie + CSRF token.
	req, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/api/auth/login", strings.NewReader(`{"username":"loginuser","password":"s3cret-pw"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", resp.StatusCode, body)
	}
	var cookieVal string
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookieVal = c.Value
			if !c.HttpOnly {
				t.Error("session cookie must be HttpOnly")
			}
		}
	}
	if cookieVal == "" {
		t.Fatal("login did not set a session cookie")
	}
	if !strings.Contains(string(body), `"csrfToken"`) || !strings.Contains(string(body), "loginuser") {
		t.Errorf("login response missing csrf/user: %s", body)
	}
	var loginResp struct {
		Session struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"session"`
	}
	if err := json.Unmarshal(body, &loginResp); err != nil || loginResp.Session.CSRFToken == "" {
		t.Fatalf("login csrf decode: csrf=%q err=%v body=%s", loginResp.Session.CSRFToken, err, body)
	}

	// The cookie authorizes /api/auth/me.
	meReq, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookieVal})
	meResp, err := h.ts.Client().Do(meReq)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	meBody, _ := io.ReadAll(meResp.Body)
	_ = meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK || !strings.Contains(string(meBody), "loginuser") {
		t.Errorf("me: status=%d body=%s", meResp.StatusCode, meBody)
	}

	// A state-changing request without the CSRF token is rejected even with a valid cookie.
	logoutReq, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookieVal})
	logoutResp, err := h.ts.Client().Do(logoutReq)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	_ = logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusForbidden {
		t.Errorf("logout without CSRF: want 403, got %d", logoutResp.StatusCode)
	}

	logoutReq, _ = http.NewRequest(http.MethodPost, h.ts.URL+"/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookieVal})
	logoutReq.Header.Set(auth.CSRFHeader, loginResp.Session.CSRFToken)
	logoutResp, err = h.ts.Client().Do(logoutReq)
	if err != nil {
		t.Fatalf("logout with csrf: %v", err)
	}
	_ = logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Errorf("logout with csrf: want 200, got %d", logoutResp.StatusCode)
	}
	meReq, _ = http.NewRequest(http.MethodGet, h.ts.URL+"/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookieVal})
	meResp, err = h.ts.Client().Do(meReq)
	if err != nil {
		t.Fatalf("me after logout: %v", err)
	}
	_ = meResp.Body.Close()
	if meResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("destroyed session after logout: want 401, got %d", meResp.StatusCode)
	}
}

func TestPasswordChangeInvalidatesOldSessionAndReturnsNewSession(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("old-secret")
	if err := h.store.Users.Create(ctx, &models.User{ID: "pw-user", Username: "pwuser", Roles: []models.Role{models.RoleViewer}}, hash); err != nil {
		t.Fatalf("create user: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/api/auth/login", strings.NewReader(`{"username":"pwuser","password":"old-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("login: want 200, got %d (%s)", resp.StatusCode, body)
	}
	var loginBody struct {
		Session struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"session"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	_ = resp.Body.Close()
	var oldCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			copied := *c
			oldCookie = &copied
		}
	}
	if oldCookie == nil || loginBody.Session.CSRFToken == "" {
		t.Fatal("login missing session cookie or csrf token")
	}

	changeReq, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/api/auth/me/password", strings.NewReader(`{"currentPassword":"old-secret","newPassword":"new-secret"}`))
	changeReq.AddCookie(oldCookie)
	changeReq.Header.Set(auth.CSRFHeader, loginBody.Session.CSRFToken)
	changeResp, err := h.ts.Client().Do(changeReq)
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if changeResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(changeResp.Body)
		_ = changeResp.Body.Close()
		t.Fatalf("change password: want 200, got %d (%s)", changeResp.StatusCode, body)
	}
	var changeBody struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(changeResp.Body).Decode(&changeBody); err != nil {
		t.Fatalf("decode change: %v", err)
	}
	_ = changeResp.Body.Close()
	var newCookie *http.Cookie
	for _, c := range changeResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			copied := *c
			newCookie = &copied
		}
	}
	if newCookie == nil || changeBody.CSRFToken == "" || changeBody.CSRFToken == loginBody.Session.CSRFToken {
		t.Fatalf("password change did not return a fresh session: cookie=%v oldCSRF=%q newCSRF=%q", newCookie, loginBody.Session.CSRFToken, changeBody.CSRFToken)
	}

	oldMe, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/api/auth/me", nil)
	oldMe.AddCookie(oldCookie)
	oldMeResp, err := h.ts.Client().Do(oldMe)
	if err != nil {
		t.Fatalf("old me: %v", err)
	}
	_ = oldMeResp.Body.Close()
	if oldMeResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old session after password change: want 401, got %d", oldMeResp.StatusCode)
	}

	newMe, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/api/auth/me", nil)
	newMe.AddCookie(newCookie)
	newMeResp, err := h.ts.Client().Do(newMe)
	if err != nil {
		t.Fatalf("new me: %v", err)
	}
	_ = newMeResp.Body.Close()
	if newMeResp.StatusCode != http.StatusOK {
		t.Fatalf("new session after password change: want 200, got %d", newMeResp.StatusCode)
	}
}

func TestDisabledUserExistingSessionRejected(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	user, err := h.store.Users.GetByID(ctx, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	user.Disabled = true
	if err := h.store.Users.Update(ctx, &user); err != nil {
		t.Fatal(err)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections", "viewer", nil); resp.Status != http.StatusUnauthorized {
		t.Fatalf("disabled user with existing session: want 401, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestPlatformAuth401IsMarked(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/api/connections", "", nil)
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request: want 401, got %d (%s)", resp.Status, resp.Body)
	}
	req, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/api/connections", nil)
	raw, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Body.Close() }()
	if got := raw.Header.Get("X-ShellCN-Auth"); got != "required" {
		t.Fatalf("X-ShellCN-Auth = %q, want required", got)
	}
}

func TestPluginRoute401IsNotMarkedAsPlatformAuth(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/api/connections/c-op/x/tester.unauth", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: h.sessions["op"].ID})
	raw, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Body.Close() }()
	if raw.StatusCode != http.StatusUnauthorized {
		t.Fatalf("plugin route unauthorized: want 401, got %d", raw.StatusCode)
	}
	if got := raw.Header.Get("X-ShellCN-Auth"); got != "" {
		t.Fatalf("X-ShellCN-Auth = %q, want empty", got)
	}
}

func TestWSRequiresTicket(t *testing.T) {
	h := newHarness(t)
	if _, err := h.dialWS(t, "op", "/api/connections/c-op/x/tester.ws"); err == nil {
		t.Error("WS without a ticket should be rejected")
	}
}

func TestWSHappyPathEcho(t *testing.T) {
	h := newHarness(t)
	tok := h.mintTicket(t, "op", "c-op", "tester.ws", nil)
	c, err := h.dialWS(t, "op", "/api/connections/c-op/x/tester.ws?ticket="+tok)
	if err != nil {
		t.Fatalf("dial with valid ticket: %v", err)
	}
	defer func() { _ = c.CloseNow() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "ping" {
		t.Errorf("echo mismatch: got %q", data)
	}
}

func TestWSAcceptsBinarySubprotocol(t *testing.T) {
	h := newHarness(t)
	tok := h.mintTicket(t, "op", "c-op", "tester.ws", nil)
	c, err := h.dialWSWithSubprotocol(t, "op", "/api/connections/c-op/x/tester.ws?ticket="+tok, "binary")
	if err != nil {
		t.Fatalf("dial with binary subprotocol: %v", err)
	}
	defer func() { _ = c.CloseNow() }()
	if got := c.Subprotocol(); got != "binary" {
		t.Fatalf("subprotocol = %q, want binary", got)
	}
}

func TestWSStreamRecordedWhenPolicyForced(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"rec","protocol":"tester","config":{"host":"h"},"recording":{"terminal":"auto"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)

	tok := h.mintTicket(t, "op", id, "tester.ws", nil)
	c, err := h.dialWS(t, "op", "/api/connections/"+id+"/x/tester.ws?ticket="+tok)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = c.Write(wctx, websocket.MessageText, []byte("ping"))
	_, _, _ = c.Read(wctx)
	_ = c.CloseNow()

	// The recording finalizes when serveStream returns; poll briefly for it.
	var recs []models.Recording
	for range 50 {
		recs, _ = h.store.Recordings.List(ctx, store.RecordingFilter{ConnectionID: id})
		if len(recs) == 1 && recs[0].Status == models.RecordingFinalized {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 recording, got %d", len(recs))
	}
	r := recs[0]
	if r.Status != models.RecordingFinalized {
		t.Fatalf("recording not finalized: %s", r.Status)
	}
	if r.Class != "terminal" || r.Format != "asciicast_v2" || r.UserID != "op" {
		t.Errorf("unexpected recording metadata: %+v", r)
	}
}

func TestWSTicketParamMismatchRejected(t *testing.T) {
	h := newHarness(t)
	// Mint a ticket bound to name=a, then try to use it for name=b.
	tok := h.mintTicket(t, "op", "c-op", "tester.ws", map[string]string{"name": "a"})
	if _, err := h.dialWS(t, "op", "/api/connections/c-op/x/tester.ws?p.name=b&ticket="+tok); err == nil {
		t.Error("ticket minted for one resource must not work for another")
	}
}

func TestWSTicketSingleUse(t *testing.T) {
	h := newHarness(t)
	tok := h.mintTicket(t, "op", "c-op", "tester.ws", nil)
	c, err := h.dialWS(t, "op", "/api/connections/c-op/x/tester.ws?ticket="+tok)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	_ = c.CloseNow()
	if _, err := h.dialWS(t, "op", "/api/connections/c-op/x/tester.ws?ticket="+tok); err == nil {
		t.Error("ticket replay must be rejected")
	}
}
