package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
)

func trimName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

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
	ds := sess.(*dockerengine.Session)

	list, err := ds.Client().ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		t.Fatalf("ContainerList: %v", err)
	}
	var id string
	for _, c := range list.Items {
		if trimName(c.Names) == name {
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

	nt, err := transport.Build(models.Connection{ID: "docker-agent", Transport: string(plugin.TransportAgent)}, reg, plugin.AgentUnix)
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
	if _, err := sess.(*dockerengine.Session).Client().ContainerList(ctx, dockerclient.ContainerListOptions{All: true, Limit: 1}); err != nil {
		t.Fatalf("ContainerList through agent transport: %v", err)
	}
}

func TestDockerEngineResourceCreateRoundTrip(t *testing.T) {
	if os.Getenv("SHELLCN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_DOCKER_INTEGRATION=1 to run against the local Docker daemon")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sess, err := Connect(ctx, plugin.ConnectConfig{
		Config: map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"},
		Net:    directNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	ds := sess.(*dockerengine.Session)

	suffix := time.Now().UTC().Format("20060102150405")
	volName := "shellcn-it-vol-" + suffix
	netName := "shellcn-it-net-" + suffix

	call := func(handler func(*plugin.RequestContext) (any, error), params map[string]string, body string) {
		t.Helper()
		rc := plugin.NewRequestContext(ctx, models.User{}, ds, params, nil, []byte(body))
		res, err := handler(rc)
		if err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ar, ok := res.(dockerengine.ActionResult); ok && !ar.OK {
			t.Fatalf("handler reported not OK")
		}
	}

	call(dockerengine.CreateVolume, nil, `{"name":"`+volName+`","driver":"local"}`)
	if _, err := ds.Client().VolumeInspect(ctx, volName, dockerclient.VolumeInspectOptions{}); err != nil {
		t.Fatalf("VolumeInspect after create: %v", err)
	}
	call(dockerengine.RemoveVolume, map[string]string{"id": volName}, "")
	if _, err := ds.Client().VolumeInspect(ctx, volName, dockerclient.VolumeInspectOptions{}); err == nil {
		t.Fatalf("volume %q still present after remove", volName)
	}

	call(dockerengine.CreateNetwork, nil, `{"name":"`+netName+`","driver":"bridge"}`)
	if _, err := ds.Client().NetworkInspect(ctx, netName, dockerclient.NetworkInspectOptions{}); err != nil {
		t.Fatalf("NetworkInspect after create: %v", err)
	}
	call(dockerengine.RemoveNetwork, map[string]string{"id": netName}, "")
	if _, err := ds.Client().NetworkInspect(ctx, netName, dockerclient.NetworkInspectOptions{}); err == nil {
		t.Fatalf("network %q still present after remove", netName)
	}

	call(dockerengine.PullImage, nil, `{"image":"alpine:3.20"}`)
	if _, err := ds.Client().ImageInspect(ctx, "alpine:3.20"); err != nil {
		t.Fatalf("ImageInspect after pull: %v", err)
	}
}

func TestDockerEngineOpsIntegrationRoundTrip(t *testing.T) {
	if os.Getenv("SHELLCN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_DOCKER_INTEGRATION=1 to run against the local Docker daemon")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	suffix := time.Now().UTC().Format("20060102150405")
	name := "shellcn-it-ops-" + suffix
	runDocker(ctx, t, "run", "-d", "--rm", "--name", name, "alpine:3.20", "sh", "-c", "while true; do sleep 1; done")
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
	ds := sess.(*dockerengine.Session)

	id := containerIDByName(ctx, t, ds, name)

	call := func(handler func(*plugin.RequestContext) (any, error), params map[string]string, body string) any {
		t.Helper()
		rc := plugin.NewRequestContext(ctx, models.User{}, ds, params, nil, []byte(body))
		res, err := handler(rc)
		if err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ar, ok := res.(dockerengine.ActionResult); ok && !ar.OK {
			t.Fatalf("handler reported not OK")
		}
		return res
	}

	// pause -> unpause
	call(dockerengine.PauseContainer, map[string]string{"id": id}, "")
	if got := containerState(ctx, t, ds, id); got != "paused" {
		t.Fatalf("after pause state = %q, want paused", got)
	}
	call(dockerengine.UnpauseContainer, map[string]string{"id": id}, "")
	if got := containerState(ctx, t, ds, id); got != "running" {
		t.Fatalf("after unpause state = %q, want running", got)
	}

	// rename
	renamed := name + "-r"
	call(dockerengine.RenameContainer, map[string]string{"id": id}, `{"name":"`+renamed+`"}`)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", renamed).Run()
	})
	if got := containerIDByName(ctx, t, ds, renamed); got != id {
		t.Fatalf("rename did not take effect: id by new name = %q, want %q", got, id)
	}

	// network connect -> disconnect
	netName := "shellcn-it-ops-net-" + suffix
	call(dockerengine.CreateNetwork, nil, `{"name":"`+netName+`","driver":"bridge"}`)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "network", "rm", netName).Run()
	})
	call(dockerengine.ConnectNetwork, map[string]string{"id": netName}, `{"container":"`+renamed+`","aliases":"alias-a"}`)
	if !containerInNetwork(ctx, t, ds, id, netName) {
		t.Fatalf("container not attached to network %q after connect", netName)
	}
	call(dockerengine.DisconnectNetwork, map[string]string{"id": netName}, `{"container":"`+renamed+`","force":true}`)
	if containerInNetwork(ctx, t, ds, id, netName) {
		t.Fatalf("container still attached to network %q after disconnect", netName)
	}

	// kill (default SIGKILL) - container uses --rm so it disappears afterward
	call(dockerengine.KillContainer, map[string]string{"id": id}, "")

	// image build -> tag -> push (validation path)
	dockerfile := "FROM alpine:3.20\nRUN echo shellcn-build > /shellcn\n"
	buildTag := "shellcn-it-build-" + suffix + ":latest"
	buildBody, _ := jsonBody(map[string]any{"tag": buildTag, "dockerfile": dockerfile})
	buildRes := call(dockerengine.BuildImage, nil, buildBody)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rmi", "-f", buildTag).Run()
	})
	if br, ok := buildRes.(dockerengine.BuildResult); !ok || !br.OK {
		t.Fatalf("build result unexpected: %#v", buildRes)
	}
	if _, err := ds.Client().ImageInspect(ctx, buildTag); err != nil {
		t.Fatalf("ImageInspect after build: %v", err)
	}

	tagTarget := "shellcn-it-tag-" + suffix + ":1.0"
	call(dockerengine.TagImage, map[string]string{"id": buildTag}, `{"target":"`+tagTarget+`"}`)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rmi", "-f", tagTarget).Run()
	})
	if _, err := ds.Client().ImageInspect(ctx, tagTarget); err != nil {
		t.Fatalf("ImageInspect after tag: %v", err)
	}

	// push: exercise the handler's validation path (empty reference must fail).
	// A real registry push is not provisioned in this environment.
	emptyRC := plugin.NewRequestContext(ctx, models.User{}, ds, nil, nil, []byte(`{"image":"  "}`))
	if _, err := dockerengine.PushImage(emptyRC); err == nil {
		t.Fatal("PushImage with empty reference should fail validation")
	}

	// compose down on a label-derived project, then up is a no-op (containers gone).
	composeName := "shellcn-it-compose-" + suffix
	runDocker(ctx, t, "run", "-d", "--name", composeName,
		"--label", "com.docker.compose.project="+composeName,
		"--label", "com.docker.compose.service=web",
		"alpine:3.20", "sh", "-c", "while true; do sleep 1; done")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", composeName).Run()
	})
	downRes := call(dockerengine.ComposeDown, map[string]string{"project": composeName}, "")
	if cr, ok := downRes.(dockerengine.ComposeResult); !ok || cr.Affected < 1 || cr.Succeeded < 1 {
		t.Fatalf("compose down result unexpected: %#v", downRes)
	}
}

func jsonBody(v map[string]any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func containerIDByName(ctx context.Context, t *testing.T, ds *dockerengine.Session, name string) string {
	t.Helper()
	list, err := ds.Client().ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		t.Fatalf("ContainerList: %v", err)
	}
	for _, c := range list.Items {
		if trimName(c.Names) == name {
			return c.ID
		}
	}
	t.Fatalf("container %q not listed", name)
	return ""
}

func containerState(ctx context.Context, t *testing.T, ds *dockerengine.Session, id string) string {
	t.Helper()
	res, err := ds.Client().ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("ContainerInspect: %v", err)
	}
	if res.Container.State == nil {
		return ""
	}
	return string(res.Container.State.Status)
}

func containerInNetwork(ctx context.Context, t *testing.T, ds *dockerengine.Session, id, netName string) bool {
	t.Helper()
	res, err := ds.Client().ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("ContainerInspect: %v", err)
	}
	if res.Container.NetworkSettings == nil {
		return false
	}
	_, ok := res.Container.NetworkSettings.Networks[netName]
	return ok
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
