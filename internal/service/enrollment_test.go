package service_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/store"
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

	enr, err := svc.Create(ctx, "agent-conn", "wss://shellcn.test/api/agent/connect")
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
	if !strings.Contains(cmd, "ghcr.io/charlesng35/shellcn-agent:latest") || strings.Contains(cmd, "shellcn-proxy") {
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

	enr, err := svc.Create(ctx, "agent-conn", "ws://localhost:5173/api/agent/connect")
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

func extractToken(t *testing.T, cmd string) string {
	t.Helper()
	re := regexp.MustCompile(`--token '([^']+)'`)
	match := re.FindStringSubmatch(cmd)
	if len(match) != 2 {
		t.Fatalf("missing token in command: %s", cmd)
	}
	return match[1]
}
