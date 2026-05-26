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
	"github.com/charlesng/shellcn/plugins/docker"
)

func TestDockerEnrollmentArtifactsAndRedeem(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(docker.New())
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "docker-agent", Name: "Docker", Protocol: "docker", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
		Config:    map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"},
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "docker-agent", "wss://shellcn.test/api/agent/connect")
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	if len(enr.Artifacts) != 1 || enr.Artifacts[0].Kind != "docker-run" {
		t.Fatalf("artifacts = %+v", enr.Artifacts)
	}
	cmd := enr.Artifacts[0].Command
	if !strings.Contains(cmd, "-e SHELLCN_ENROLL_TOKEN=") || strings.Contains(cmd, "/api/agent/connect/") {
		t.Fatalf("artifact should use token env and not token-in-path: %s", cmd)
	}
	if !strings.Contains(cmd, "ghcr.io/charlesng35/shellcn-agent:latest") || strings.Contains(cmd, "shellcn-proxy") {
		t.Fatalf("artifact should use the published shellcn-agent image: %s", cmd)
	}
	token := extractToken(t, cmd)
	connectionID, proxy, err := svc.Redeem(ctx, token)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if connectionID != "docker-agent" || proxy.Mode != plugin.AgentUnix || proxy.Address != "/var/run/docker.sock" || proxy.Risk != plugin.RiskPrivileged {
		t.Fatalf("redeem target mismatch: connection=%q proxy=%+v", connectionID, proxy)
	}
	if _, _, err := svc.Redeem(ctx, token); err != service.ErrEnrollmentInvalid {
		t.Fatalf("token replay error = %v, want ErrEnrollmentInvalid", err)
	}
	if state := svc.State(ctx, "docker-agent"); state.Status != string(models.EnrollmentOnline) {
		t.Fatalf("state after redeem = %+v", state)
	}
	svc.MarkOffline(ctx, "docker-agent")
	if state := svc.State(ctx, "docker-agent"); state.Status != string(models.EnrollmentOffline) {
		t.Fatalf("state after offline = %+v", state)
	}
}

func extractToken(t *testing.T, cmd string) string {
	t.Helper()
	re := regexp.MustCompile(`SHELLCN_ENROLL_TOKEN=([^ ]+)`)
	match := re.FindStringSubmatch(cmd)
	if len(match) != 2 {
		t.Fatalf("missing token in command: %s", cmd)
	}
	return match[1]
}
