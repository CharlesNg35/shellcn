package swarm

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/swarm"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
)

// TestSwarmPluginIntegration provisions an isolated swarm inside a docker:dind
// container (so it never disturbs the host daemon), then drives the swarm
// orchestration handlers against it: service update (scale + image), service
// inspect, and a node drain/active round-trip.
func TestSwarmPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_SWARM_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_SWARM_INTEGRATION=1 to run against a live swarm")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	suffix := time.Now().UTC().Format("20060102150405")
	dind := "shellcn-swarm-dind-" + suffix
	// Map the dind daemon's insecure TCP port to a host port so the plugin can
	// dial it via dockerengine's TCP endpoint.
	const hostPort = "23750"
	runHost(ctx, t, "run", "-d", "--rm", "--privileged",
		"--name", dind,
		"-p", hostPort+":2375",
		"-e", "DOCKER_TLS_CERTDIR=",
		"docker:dind", "dockerd", "--host=tcp://0.0.0.0:2375", "--host=unix:///var/run/docker.sock")
	t.Cleanup(func() {
		cleanupCtx, cc := context.WithTimeout(context.Background(), 30*time.Second)
		defer cc()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", dind).Run()
	})

	// Wait for the inner daemon to accept connections, then init a swarm.
	waitDaemon(ctx, t, dind)
	execDind(ctx, t, dind, "docker", "swarm", "init")

	port, _ := strconv.Atoi(hostPort)
	sess, err := Connect(ctx, plugin.ConnectConfig{
		Config: map[string]any{"endpoint_type": "tcp", "host": "127.0.0.1", "port": port},
		Net:    directNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	ds := sess.(*dockerengine.Session)
	cli := ds.Client()

	// Create a replicated test service directly via the client (the plugin has no
	// create handler), then exercise the update handler against it.
	svcName := "shellcn-it-svc"
	created, err := cli.ServiceCreate(ctx, dockerclient.ServiceCreateOptions{Spec: swarm.ServiceSpec{
		Annotations:  swarm.Annotations{Name: svcName},
		TaskTemplate: swarm.TaskSpec{ContainerSpec: &swarm.ContainerSpec{Image: "nginx:1.27-alpine"}},
		Mode:         swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: u64(1)}},
	}})
	if err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}

	call := func(handler func(*plugin.RequestContext) (any, error), params map[string]string, body string) {
		t.Helper()
		rc := plugin.NewRequestContext(ctx, models.User{ID: "it"}, ds, params, nil, []byte(body))
		res, err := handler(rc)
		if err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ar, ok := res.(dockerengine.ActionResult); ok && !ar.OK {
			t.Fatalf("handler reported not OK")
		}
	}

	// Update: scale to 3 replicas and change the image.
	call(updateService, map[string]string{"id": created.ID}, `{"image":"nginx:1.27-alpine","replicas":3}`)
	insp, err := cli.ServiceInspect(ctx, created.ID, dockerclient.ServiceInspectOptions{})
	if err != nil {
		t.Fatalf("ServiceInspect after update: %v", err)
	}
	if got := *insp.Service.Spec.Mode.Replicated.Replicas; got != 3 {
		t.Fatalf("replicas after update = %d, want 3", got)
	}

	// Node drain/active round-trip against the single manager node.
	nodes, err := cli.NodeList(ctx, dockerclient.NodeListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		t.Fatalf("NodeList: %v (n=%d)", err, len(nodes.Items))
	}
	nodeID := nodes.Items[0].ID

	call(updateNode, map[string]string{"id": nodeID}, `{"availability":"drain"}`)
	if got := nodeAvailability(ctx, t, cli, nodeID); got != swarm.NodeAvailabilityDrain {
		t.Fatalf("availability after drain = %q, want drain", got)
	}
	call(updateNode, map[string]string{"id": nodeID}, `{"availability":"active"}`)
	if got := nodeAvailability(ctx, t, cli, nodeID); got != swarm.NodeAvailabilityActive {
		t.Fatalf("availability after active = %q, want active", got)
	}

	_, _ = cli.ServiceRemove(ctx, created.ID, dockerclient.ServiceRemoveOptions{})
}

func nodeAvailability(ctx context.Context, t *testing.T, cli *dockerclient.Client, id string) swarm.NodeAvailability {
	t.Helper()
	res, err := cli.NodeInspect(ctx, id, dockerclient.NodeInspectOptions{})
	if err != nil {
		t.Fatalf("NodeInspect: %v", err)
	}
	return res.Node.Spec.Availability
}

func runHost(ctx context.Context, t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func execDind(ctx context.Context, t *testing.T, name string, args ...string) {
	t.Helper()
	full := append([]string{"exec", name}, args...)
	out, err := exec.CommandContext(ctx, "docker", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec %s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

// waitDaemon polls the inner daemon until `docker info` succeeds inside the dind
// container.
func waitDaemon(ctx context.Context, t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		c, cc := context.WithTimeout(ctx, 5*time.Second)
		err := exec.CommandContext(c, "docker", "exec", name, "docker", "info").Run()
		cc()
		if err == nil {
			return
		}
		t2, cc2 := context.WithTimeout(ctx, time.Second)
		<-t2.Done()
		cc2()
	}
	t.Fatalf("dind daemon %q never became ready", name)
}
