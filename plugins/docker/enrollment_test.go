package docker

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
)

func TestEnrollmentCommandUsesPublishedAgentImage(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(New())
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
		"docker run --rm --name shellcn-agent",
		`--group-add "$(stat -c '%g' /var/run/docker.sock)"`,
		"-e SHELLCN_CONNECT_URL='wss://shellcn.test/api/agent/connect'",
		"-e SHELLCN_ENROLL_TOKEN='",
		"'/var/run/docker.sock:/var/run/docker.sock'",
		"'ghcr.io/charlesng35/shellcn-agent:latest'",
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
	reg := plugin.NewRegistry()
	reg.MustRegister(New())
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
