package service_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/plugins/docker"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestEnrollmentCommandUsesPublishedAgentImage(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := pluginregistry.New()
	reg.MustRegister(docker.New())
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "docker-agent", Name: "Docker", Protocol: "docker", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "docker-agent", "wss://shellcn.test/api/agent/connect", nil)
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	cmd := enr.Artifacts[0].Command
	for _, want := range []string{
		"docker run --rm --name " + app.AgentBinary,
		"--network host",
		`--group-add "$(stat -c '%g' /var/run/docker.sock)"`,
		"-e SHELLCN_CONNECT_URL='wss://shellcn.test/api/agent/connect'",
		"-e SHELLCN_ENROLL_TOKEN='",
		"'/var/run/docker.sock:/var/run/docker.sock'",
		"'" + app.AgentImageLatest + "'",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("command missing %q: %s", want, cmd)
		}
	}
	if strings.Contains(cmd, "shellcn-proxy") || strings.Contains(cmd, "SHELLCN_INSECURE=1") || strings.Contains(cmd, "host.docker.internal") {
		t.Fatalf("production command contains unexpected dev/proxy content: %s", cmd)
	}
}

func TestEnrollmentCommandAdaptsLocalhostForContainer(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := pluginregistry.New()
	reg.MustRegister(docker.New())
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "docker-agent", Name: "Docker", Protocol: "docker", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "docker-agent", "ws://localhost:5173/api/agent/connect", nil)
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	cmd := enr.Artifacts[0].Command
	for _, want := range []string{
		"--add-host=host.docker.internal:host-gateway",
		"-e SHELLCN_CONNECT_URL='ws://host.docker.internal:5173/api/agent/connect'",
		"-e SHELLCN_INSECURE=1",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("local command missing %q: %s", want, cmd)
		}
	}
	if strings.Contains(cmd, "ws://localhost:5173") {
		t.Fatalf("command should not point at container-local localhost: %s", cmd)
	}
	if !regexp.MustCompile(`SHELLCN_ENROLL_TOKEN='[^']+'`).MatchString(cmd) {
		t.Fatalf("command missing quoted enrollment token: %s", cmd)
	}
}

func TestEnrollmentOffersComposeFile(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := pluginregistry.New()
	reg.MustRegister(docker.New())
	if err := st.Connections.Create(ctx, &models.Connection{
		ID: "docker-agent", Name: "Docker", Protocol: "docker", OwnerID: "owner",
		Transport: string(plugin.TransportAgent),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	svc := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)

	enr, err := svc.Create(ctx, "docker-agent", "wss://shellcn.test/api/agent/connect", nil)
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	var compose *service.InstallArtifact
	for i := range enr.Artifacts {
		if enr.Artifacts[i].Kind == "docker-compose" {
			compose = &enr.Artifacts[i]
		}
	}
	if compose == nil || compose.Filename != "shellcn-agent.compose.yml" {
		t.Fatalf("compose artifact missing or unnamed: %+v", compose)
	}
	for _, want := range []string{
		"network_mode: host",
		`SHELLCN_CONNECT_URL: "wss://shellcn.test/api/agent/connect"`,
		`SHELLCN_ENROLL_TOKEN: "`,
	} {
		if !strings.Contains(compose.Content, want) {
			t.Fatalf("compose content missing %q:\n%s", want, compose.Content)
		}
	}
}
