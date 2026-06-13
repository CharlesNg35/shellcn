package cluster

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverInternalURLUsesPlatformEnvHost(t *testing.T) {
	clearDiscoveryEnv(t)
	t.Setenv("POD_IP", "10.42.0.15")
	if got, want := DiscoverInternalURL("8081", false), "http://10.42.0.15:8081"; got != want {
		t.Fatalf("internal URL = %q, want %q", got, want)
	}
}

func TestDiscoverInternalURLIgnoresLoopbackEnvHost(t *testing.T) {
	clearDiscoveryEnv(t)
	t.Setenv("POD_IP", "127.0.0.1")
	if got := DiscoverInternalURL("8081", false); strings.Contains(got, "127.0.0.1") || strings.Contains(got, "localhost") {
		t.Fatalf("internal URL must not use loopback host, got %q", got)
	}
}

func TestDiscoverInternalURLUsesECSMetadata(t *testing.T) {
	clearDiscoveryEnv(t)
	srv := newMetadataServer(t, `{"Networks":[{"IPv4Addresses":["172.20.4.10"]}]}`)
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", srv)
	if got, want := DiscoverInternalURL("8081", true), "https://172.20.4.10:8081"; got != want {
		t.Fatalf("internal URL = %q, want %q", got, want)
	}
}

func TestPortFromListenAddress(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want string
	}{
		{":8081", "8081"},
		{"127.0.0.1:8081", "8081"},
		{"0.0.0.0:8081", "8081"},
		{"[::]:8081", "8081"},
		{"shellcn.local:9443", "9443"},
	} {
		if got := PortFromListenAddress(tc.addr); got != tc.want {
			t.Fatalf("PortFromListenAddress(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func newMetadataServer(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func clearDiscoveryEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SHELLCN_INSTANCE_IP",
		"POD_IP",
		"KUBERNETES_POD_IP",
		"MY_POD_IP",
		"CONTAINER_IP",
		"HOST_IP",
		"ECS_CONTAINER_METADATA_URI_V4",
	} {
		t.Setenv(key, "")
	}
}
