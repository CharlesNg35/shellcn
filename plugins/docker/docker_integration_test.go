package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
)

func TestDockerPluginIntegrationLocalDaemon(t *testing.T) {
	if os.Getenv("SHELLCN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_DOCKER_INTEGRATION=1 to run against the local Docker daemon")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := "shellcn-it-" + time.Now().UTC().Format("20060102150405")
	runDocker(ctx, t, "run", "-d", "--rm", "--name", name, "alpine:3.20", "sh", "-c", "while true; do echo shellcn-log; sleep 1; done")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})

	sess, err := Connect(ctx, plugin.ConnectConfig{
		Config: map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"},
		Net:    directNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	ds := sess.(*Session)

	list, err := ds.cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		t.Fatalf("ContainerList: %v", err)
	}
	var id string
	for _, c := range list.Items {
		if firstName(c.Names, "") == name {
			id = c.ID
			break
		}
	}
	if id == "" {
		t.Fatalf("test container %q not listed", name)
	}

	logs, err := ds.OpenChannel(ctx, plugin.ChannelRequest{Kind: plugin.StreamLogs, Params: map[string]string{"id": id, "tail": "5", "follow": "false", "timestamps": "false"}})
	if err != nil {
		t.Fatalf("open logs: %v", err)
	}
	logText := readUntil(t, logs, "shellcn-log")
	_ = logs.Close()
	if !strings.Contains(logText, "shellcn-log") {
		t.Fatalf("logs did not include expected line: %q", logText)
	}

	execCh, err := ds.OpenChannel(ctx, plugin.ChannelRequest{Kind: plugin.StreamTerminal, Params: map[string]string{"id": id, "command": "echo shellcn-exec"}})
	if err != nil {
		t.Fatalf("open exec: %v", err)
	}
	out := readUntil(t, execCh, "shellcn-exec")
	_ = execCh.Close()
	if !strings.Contains(out, "shellcn-exec") {
		t.Fatalf("exec output did not include expected line: %q", out)
	}
}

func TestDockerPluginIntegrationAgentTransport(t *testing.T) {
	if os.Getenv("SHELLCN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_DOCKER_INTEGRATION=1 to run against the local Docker daemon")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	reg := transport.NewRegistry()
	release := reg.Register("docker-agent", func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", "/var/run/docker.sock")
	})
	defer release()

	nt, err := transport.Build(models.Connection{ID: "docker-agent", Transport: string(plugin.TransportAgent)}, reg)
	if err != nil {
		t.Fatalf("agent transport build: %v", err)
	}
	sess, err := Connect(ctx, plugin.ConnectConfig{
		ConnectionID: "docker-agent",
		Transport:    plugin.TransportAgent,
		Config:       map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"},
		Net:          nt,
	})
	if err != nil {
		t.Fatalf("Connect through agent transport: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.(*Session).cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true, Limit: 1}); err != nil {
		t.Fatalf("ContainerList through agent transport: %v", err)
	}
}

func runDocker(ctx context.Context, t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func readUntil(t *testing.T, r io.Reader, needle string) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		tmp := make([]byte, 256)
		deadline := time.After(8 * time.Second)
		for {
			select {
			case <-deadline:
				done <- buf.String()
				return
			default:
			}
			n, err := r.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
				if strings.Contains(buf.String(), needle) {
					done <- buf.String()
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					done <- buf.String()
					return
				}
				done <- buf.String()
				return
			}
		}
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %q", needle)
		return ""
	}
}
