package kubernetes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// captureClient is a ClientStream that records server→client bytes and feeds
// optional client→server input.
type captureClient struct {
	mu  sync.Mutex
	out strings.Builder
	in  io.Reader
	ctx context.Context
}

func (c *captureClient) Read(p []byte) (int, error) {
	if c.in == nil {
		<-c.ctx.Done()
		return 0, io.EOF
	}
	return c.in.Read(p)
}

func (c *captureClient) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.WriteString(string(p))
}

func (c *captureClient) Close() error             { return nil }
func (c *captureClient) Context() context.Context { return c.ctx }

func TestLogsStream(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web-1", "namespace": "default"},
			"spec":     obj{"containers": []obj{{"name": "app"}}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1/log", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "line one\nline two\n")
	})
	sess := connectTo(t, mux)

	cc := &captureClient{ctx: context.Background()}
	rcx := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess,
		map[string]string{"namespace": "default", "name": "web-1", "follow": "false"}, nil, nil)
	if err := LogsStream(rcx, cc); err != nil {
		t.Fatalf("logs: %v", err)
	}
	if got := cc.out.String(); !strings.Contains(got, "line one") || !strings.Contains(got, "line two") {
		t.Fatalf("logs output = %q", got)
	}
}

func TestLogsStreamPrefixesAllContainers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web-1", "namespace": "default"},
			"spec":     obj{"containers": []obj{{"name": "app"}, {"name": "sidecar"}}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1/log", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Query().Get("container")+"-line\n")
	})
	sess := connectTo(t, mux)

	cc := &captureClient{ctx: context.Background()}
	rcx := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess,
		map[string]string{"namespace": "default", "name": "web-1", "follow": "false"}, nil, nil)
	if err := LogsStream(rcx, cc); err != nil {
		t.Fatalf("logs: %v", err)
	}
	got := cc.out.String()
	if !strings.Contains(got, "[app] app-line") || !strings.Contains(got, "[sidecar] sidecar-line") {
		t.Fatalf("multi-container logs not prefixed per container: %q", got)
	}
}

func TestPodContainersOffersAllAndEach(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web-1", "namespace": "default"},
			"spec":     obj{"containers": []obj{{"name": "app"}, {"name": "sidecar"}}},
		})
	})
	sess := connectTo(t, mux)

	// merge=true (logs) prepends "All containers".
	out, err := PodContainers(rc(sess, map[string]string{"namespace": "default", "name": "web-1", "merge": "true"}))
	if err != nil {
		t.Fatalf("containers: %v", err)
	}
	page := out.(plugin.Page[plugin.Option])
	if len(page.Items) != 3 || page.Items[0].Value != "" || page.Items[0].Label != "All containers" {
		t.Fatalf("merged options = %+v, want [All, app, sidecar]", page.Items)
	}

	// Without merge (files) the picker lists only the containers.
	out, err = PodContainers(rc(sess, map[string]string{"namespace": "default", "name": "web-1"}))
	if err != nil {
		t.Fatalf("containers: %v", err)
	}
	if page = out.(plugin.Page[plugin.Option]); len(page.Items) != 2 || page.Items[0].Value != "app" {
		t.Fatalf("plain options = %+v, want [app, sidecar]", page.Items)
	}
}

func TestWorkloadLogsStreamAggregatesSelectedPods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/apps/v1/namespaces/default/deployments/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": obj{
				"name":      "web",
				"namespace": "default",
			},
			"spec": obj{"selector": obj{"matchLabels": obj{"app": "web"}}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1",
			"kind":       "PodList",
			"items": []obj{
				{
					"metadata": obj{"name": "web-1", "namespace": "default"},
					"spec":     obj{"containers": []obj{{"name": "app"}}},
				},
				{
					"metadata": obj{"name": "web-2", "namespace": "default"},
					"spec":     obj{"containers": []obj{{"name": "app"}}},
				},
			},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1/log", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("container"); got != "app" {
			t.Fatalf("web-1 container = %q, want app", got)
		}
		_, _ = io.WriteString(w, "one\n")
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-2/log", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("container"); got != "app" {
			t.Fatalf("web-2 container = %q, want app", got)
		}
		_, _ = io.WriteString(w, "two\n")
	})
	sess := connectTo(t, mux)

	cc := &captureClient{ctx: context.Background()}
	rcx := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess,
		map[string]string{"kind": "deployment", "namespace": "default", "name": "web", "follow": "false"}, nil, nil)
	if err := WorkloadLogsStream(rcx, cc); err != nil {
		t.Fatalf("workload logs: %v", err)
	}
	got := cc.out.String()
	for _, want := range []string{"[web-1/app] one", "[web-2/app] two"} {
		if !strings.Contains(got, want) {
			t.Fatalf("workload logs = %q, missing %q", got, want)
		}
	}
}

func TestPodExecutorBuilds(t *testing.T) {
	sess := connectTo(t, http.NewServeMux()).(*Session)
	exec, err := sess.podExecutor("default", "web-1", &corev1.PodExecOptions{
		Command: []string{"/bin/sh"}, Stdin: true, Stdout: true, TTY: true,
	})
	if err != nil || exec == nil {
		t.Fatalf("podExecutor = %v, %v", exec, err)
	}
}

func TestExecContainerResolvesDefault(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{
				"name": "web-1", "namespace": "default",
				"annotations": obj{"kubectl.kubernetes.io/default-container": "sidecar"},
			},
			"spec": obj{"containers": []obj{{"name": "nginx"}, {"name": "sidecar"}}},
		})
	})
	sess := connectTo(t, mux).(*Session)

	// An explicit container is returned without a pod lookup.
	if got, err := sess.execContainer(context.Background(), "default", "web-1", "nginx"); err != nil || got != "nginx" {
		t.Fatalf("explicit = %q, %v", got, err)
	}
	// No container falls back to the default-container annotation.
	if got, err := sess.execContainer(context.Background(), "default", "web-1", ""); err != nil || got != "sidecar" {
		t.Fatalf("default = %q, %v, want sidecar (annotation)", got, err)
	}
}

func TestExecContainerDefaultsToFirstWithoutAnnotation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web-1", "namespace": "default"},
			"spec":     obj{"containers": []obj{{"name": "nginx"}, {"name": "sidecar"}}},
		})
	})
	sess := connectTo(t, mux).(*Session)

	if got, err := sess.execContainer(context.Background(), "default", "web-1", ""); err != nil || got != "nginx" {
		t.Fatalf("default = %q, %v, want first container nginx", got, err)
	}
}

func TestStreamExecWritesFullErrorToTerminal(t *testing.T) {
	msg := strings.Repeat("failed to exec in container: ", 8) + `exec: "/bin/sh": stat /bin/sh: no such file or directory`
	cc := &captureClient{ctx: context.Background(), in: strings.NewReader("")}

	err := streamExec(cc, fakeExecutor{err: errors.New(msg)}, true, 0, 0)
	if err == nil {
		t.Fatal("streamExec error = nil")
	}
	if got := cc.out.String(); !strings.Contains(got, msg) {
		t.Fatalf("terminal error = %q, want full message", got)
	}
}

func TestStreamExecRetriesMissingDefaultShell(t *testing.T) {
	cc := &captureClient{ctx: context.Background(), in: strings.NewReader("")}
	calls := 0

	err := streamExecCommands(cc, func(command []string) (remotecommand.Executor, error) {
		calls++
		switch calls {
		case 1:
			return fakeExecutor{err: errors.New(`exec: "/bin/sh": stat /bin/sh: no such file or directory`)}, nil
		default:
			if command[0] != "/bin/bash" {
				t.Fatalf("fallback command = %v, want /bin/bash", command)
			}
			return fakeExecutor{out: "shell ready\n"}, nil
		}
	}, [][]string{{"/bin/sh", "-c", "x"}, {"/bin/bash", "-lc", "x"}}, true, 0, 0)
	if err != nil {
		t.Fatalf("streamExecCommands: %v", err)
	}
	if calls != 2 {
		t.Fatalf("exec attempts = %d, want 2", calls)
	}
	if got := cc.out.String(); got != "shell ready\n" {
		t.Fatalf("terminal output = %q", got)
	}
}

type fakeExecutor struct {
	out string
	err error
}

func (f fakeExecutor) Stream(options remotecommand.StreamOptions) error {
	return f.StreamWithContext(context.Background(), options)
}

func (f fakeExecutor) StreamWithContext(_ context.Context, options remotecommand.StreamOptions) error {
	if f.out != "" {
		_, _ = options.Stdout.Write([]byte(f.out))
	}
	return f.err
}
