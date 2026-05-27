package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

// fakeNet is a NetTransport whose HTTP() points client-go at a test server,
// standing in for the L7 agent's reverse proxy.
type fakeNet struct{ baseURL string }

func (f fakeNet) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, fmt.Errorf("dial not used")
}

func (f fakeNet) HTTP() (string, http.RoundTripper, bool) {
	return f.baseURL, http.DefaultTransport, true
}

// fakeAPIServer answers the calls Connect + ListNamespaces make.
func fakeAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	nsList := corev1.NamespaceList{
		TypeMeta: metav1.TypeMeta{Kind: "NamespaceList", APIVersion: "v1"},
		Items: []corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
			{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"major": "1", "minor": "31", "gitVersion": "v1.31.0"})
	})
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, nsList)
	})
	mux.HandleFunc("/api/v1/namespaces/default", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, nsList.Items[0])
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func kubeconfigFor(server string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: %s
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`, server)
}

func listNamespaces(t *testing.T, sess plugin.Session) []Row {
	t.Helper()
	rc := plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sess, nil, url.Values{}, nil)
	out, err := ListNamespaces(rc)
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	page, ok := out.(plugin.Page[Row])
	if !ok {
		t.Fatalf("ListNamespaces returned %T, want Page[Row]", out)
	}
	return page.Items
}

func TestListNamespacesDirectKubeconfig(t *testing.T) {
	srv := fakeAPIServer(t)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1",
		Transport:    plugin.TransportDirect,
		Config:       map[string]any{"kubeconfig": kubeconfigFor(srv.URL)},
		Net:          fakeNet{},
	})
	if err != nil {
		t.Fatalf("Connect direct: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rows := listNamespaces(t, sess)
	if len(rows) != 2 || rows[0]["name"] != "default" {
		t.Fatalf("direct namespaces = %v", rows)
	}
}

func TestListNamespacesAgentL7(t *testing.T) {
	srv := fakeAPIServer(t)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1",
		Transport:    plugin.TransportAgent,
		Config:       map[string]any{},
		Net:          fakeNet{baseURL: srv.URL},
	})
	if err != nil {
		t.Fatalf("Connect agent: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rows := listNamespaces(t, sess)
	if len(rows) != 2 || rows[1]["name"] != "kube-system" {
		t.Fatalf("agent namespaces = %v", rows)
	}
}

func TestBuildRESTConfigDirectRequiresKubeconfig(t *testing.T) {
	_, err := buildRESTConfig(plugin.ConnectConfig{Transport: plugin.TransportDirect, Config: map[string]any{}})
	if err == nil {
		t.Fatal("direct transport without a kubeconfig should error")
	}
}

func TestBuildRESTConfigRejectsExecCredential(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: test
  context: {cluster: test, user: test}
current-context: test
users:
- name: test
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: /bin/evil
`
	_, err := buildRESTConfig(plugin.ConnectConfig{Transport: plugin.TransportDirect, Config: map[string]any{"kubeconfig": kubeconfig}})
	if err == nil {
		t.Fatal("kubeconfig with an exec credential plugin must be rejected (arbitrary code execution risk)")
	}
}

func TestBuildRESTConfigAgentRequiresL7(t *testing.T) {
	// A direct NetTransport reports HTTP() ok=false; agent k8s must reject it.
	_, err := buildRESTConfig(plugin.ConnectConfig{Transport: plugin.TransportAgent, Net: noHTTPNet{}})
	if err == nil {
		t.Fatal("agent transport without an L7 endpoint must error")
	}
}

type noHTTPNet struct{}

func (noHTTPNet) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, fmt.Errorf("nope")
}
func (noHTTPNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("kubernetes manifest invalid: %v", err)
	}
}

func TestManifestDeclaresGenericL7Agent(t *testing.T) {
	m := New().Manifest()
	if m.Agent == nil || m.Agent.Proxy.Mode != plugin.AgentHTTP {
		t.Fatal("kubernetes must declare an http_proxy agent")
	}
	if m.Agent.Proxy.TokenFile == "" || m.Agent.Proxy.CAFile == "" {
		t.Fatal("k8s agent ProxyTarget must declare the in-cluster token/CA files (kept out of the generic agent)")
	}
}
