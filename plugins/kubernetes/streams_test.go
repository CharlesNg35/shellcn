package kubernetes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
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
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1/log", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "line one\nline two\n")
	})
	sess := connectTo(t, mux)

	cc := &captureClient{ctx: context.Background()}
	rcx := plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sess,
		map[string]string{"namespace": "default", "name": "web-1", "follow": "false"}, nil, nil)
	if err := LogsStream(rcx, cc); err != nil {
		t.Fatalf("logs: %v", err)
	}
	if got := cc.out.String(); !strings.Contains(got, "line one") || !strings.Contains(got, "line two") {
		t.Fatalf("logs output = %q", got)
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
