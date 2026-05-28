package service_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
)

type agentTestPlugin struct{}

func (agentTestPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "agenttest",
		Title:      "Agent Test",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
			plugin.TransportAgent,
		},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentTCP, Address: "127.0.0.1:1", Risk: plugin.RiskPrivileged},
			Install: []plugin.InstallArtifact{{
				Label:      "Container",
				Kind:       "container-run",
				ConnectURL: plugin.ArtifactConnectURL{LocalhostHost: "host.container.internal"},
				Template: "run {{if .LocalhostHostRequired}}--host {{.LocalhostHost}} {{end}}" +
					"--connect {{shellquote .ConnectURL}} " +
					"{{if .Insecure}}--insecure {{end}}" +
					"--token {{shellquote .Token}} " +
					"--image {{shellquote .Image}}",
			}},
		},
		Tabs: []plugin.Tab{{Key: "main", Label: "Main", Panel: plugin.PanelDocument}},
	}
}

func (agentTestPlugin) Routes() []plugin.Route { return nil }

func (agentTestPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, plugin.ErrNotSupported
}

func TestEnrollmentArtifactsAndRedeem(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(agentTestPlugin{})
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "agent-conn", Name: "Agent", Protocol: "agenttest", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "agent-conn", "wss://shellcn.test/api/agent/connect", nil)
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	if len(enr.Artifacts) != 1 || enr.Artifacts[0].Kind != "container-run" {
		t.Fatalf("artifacts = %+v", enr.Artifacts)
	}
	cmd := enr.Artifacts[0].Command
	if !strings.Contains(cmd, "--token '") || strings.Contains(cmd, "/api/agent/connect/") {
		t.Fatalf("artifact should use token env and not token-in-path: %s", cmd)
	}
	if !strings.Contains(cmd, app.AgentImageLatest) || strings.Contains(cmd, "shellcn-proxy") {
		t.Fatalf("artifact should use the published shellcn-agent image: %s", cmd)
	}
	if strings.Contains(cmd, "--insecure") {
		t.Fatalf("wss enrollment should not enable insecure mode: %s", cmd)
	}
	token := extractToken(t, cmd)
	connectionID, proxy, err := svc.Redeem(ctx, token)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if connectionID != "agent-conn" || proxy.Mode != plugin.AgentTCP || proxy.Address != "127.0.0.1:1" || proxy.Risk != plugin.RiskPrivileged {
		t.Fatalf("redeem target mismatch: connection=%q proxy=%+v", connectionID, proxy)
	}
	if state := svc.State(ctx, "agent-conn"); state.Status != string(models.EnrollmentOnline) {
		t.Fatalf("state after redeem = %+v", state)
	}
	if _, _, err := svc.Redeem(ctx, token); err != nil {
		t.Fatalf("active agent should be able to reconnect with same token: %v", err)
	}
	svc.MarkOffline(ctx, "agent-conn")
	if state := svc.State(ctx, "agent-conn"); state.Status != string(models.EnrollmentOffline) {
		t.Fatalf("state after offline = %+v", state)
	}
	if state := svc.State(ctx, "agent-conn"); state.Message != "Agent disconnected." {
		t.Fatalf("offline state should explain the disconnect: %+v", state)
	}
	if _, _, err := svc.Redeem(ctx, token); err != nil {
		t.Fatalf("offline agent should reconnect with same token: %v", err)
	}
	if state := svc.State(ctx, "agent-conn"); state.Status != string(models.EnrollmentOnline) {
		t.Fatalf("state after reconnect = %+v", state)
	}
}

func TestEnrollmentLocalDevContainerCommand(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(agentTestPlugin{})
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "agent-conn", Name: "Agent", Protocol: "agenttest", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "agent-conn", "ws://localhost:5173/api/agent/connect", nil)
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	cmd := enr.Artifacts[0].Command
	for _, want := range []string{
		"--host host.container.internal",
		"--connect 'ws://host.container.internal:5173/api/agent/connect'",
		"--insecure",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("dev command missing %q: %s", want, cmd)
		}
	}
	if strings.Contains(cmd, "ws://localhost:5173") {
		t.Fatalf("container command should not point back at container-local localhost: %s", cmd)
	}
}

type urlArtifactPlugin struct{}

func (urlArtifactPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "urlagent",
		Title:      "URL Agent",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
			plugin.TransportAgent,
		},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentHTTP, Address: "https://api.internal", Risk: plugin.RiskPrivileged, TokenFile: "/t", CAFile: "/ca"},
			Install: []plugin.InstallArtifact{{
				Label:    "Manifest",
				Kind:     "manifest",
				Delivery: plugin.DeliveryURL,
				Template: `apply -f "{{.ArtifactURL}}"`,
				Content:  "token={{.Token}}\nconnect={{.ConnectURL}}\n",
			}},
		},
		Tabs: []plugin.Tab{{Key: "main", Label: "Main", Panel: plugin.PanelDocument}},
	}
}

func (urlArtifactPlugin) Routes() []plugin.Route { return nil }
func (urlArtifactPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, plugin.ErrNotSupported
}

func TestURLDeliveredArtifactLazyMint(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(urlArtifactPlugin{})
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "url-conn", Name: "URL", Protocol: "urlagent", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	var gotKind string
	minter := func(enrollmentID, kind string) (string, error) {
		gotKind = kind
		return "https://gw.test/api/connections/url-conn/agent/enrollments/" + enrollmentID + "/artifacts/" + kind + "?ticket=T", nil
	}
	enr, err := svc.Create(ctx, "url-conn", "wss://gw.test/api/agent/connect", minter)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotKind != "manifest" {
		t.Fatalf("minter kind = %q", gotKind)
	}
	cmd := enr.Artifacts[0].Command
	if !strings.Contains(cmd, "?ticket=T") || enr.Artifacts[0].URL == "" {
		t.Fatalf("url artifact command should reference the fetch URL: %q (url=%q)", cmd, enr.Artifacts[0].URL)
	}
	if strings.Contains(cmd, "token=") {
		t.Fatalf("url artifact command must not carry the token: %q", cmd)
	}

	// The body is rendered (and the real token minted) only at fetch time.
	content, err := svc.RenderArtifactContent(ctx, "url-conn", enr.EnrollmentID, "manifest", "wss://gw.test/api/agent/connect")
	if err != nil {
		t.Fatalf("render content: %v", err)
	}
	if !strings.Contains(content, "connect=wss://gw.test/api/agent/connect") {
		t.Fatalf("content missing connect URL: %q", content)
	}
	token := tokenFromContent(t, content)

	// The minted token is now redeemable as the agent credential.
	connID, proxy, err := svc.Redeem(ctx, token)
	if err != nil || connID != "url-conn" || proxy.Mode != plugin.AgentHTTP {
		t.Fatalf("redeem minted token: conn=%q proxy=%+v err=%v", connID, proxy, err)
	}
}

func TestRenderArtifactContentRejectsWrongConnection(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(urlArtifactPlugin{})
	_ = st.Connections.Create(ctx, &models.Connection{ID: "url-conn", Protocol: "urlagent", OwnerID: "o", Transport: string(plugin.TransportAgent)})
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)
	enr, err := svc.Create(ctx, "url-conn", "wss://gw.test/api/agent/connect", func(_, _ string) (string, error) { return "https://x?ticket=T", nil })
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.RenderArtifactContent(ctx, "other-conn", enr.EnrollmentID, "manifest", "wss://gw.test"); err == nil {
		t.Fatal("render must reject a mismatched connection id")
	}
	if _, err := svc.RenderArtifactContent(ctx, "url-conn", enr.EnrollmentID, "unknown", "wss://gw.test"); err == nil {
		t.Fatal("render must reject an unknown artifact kind")
	}
}

func tokenFromContent(t *testing.T, content string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		if rest, ok := strings.CutPrefix(line, "token="); ok {
			return rest
		}
	}
	t.Fatalf("no token in content: %q", content)
	return ""
}

func extractToken(t *testing.T, cmd string) string {
	t.Helper()
	re := regexp.MustCompile(`--token '([^']+)'`)
	match := re.FindStringSubmatch(cmd)
	if len(match) != 2 {
		t.Fatalf("missing token in command: %s", cmd)
	}
	return match[1]
}
