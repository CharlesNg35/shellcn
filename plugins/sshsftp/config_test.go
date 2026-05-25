package sshsftp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestConnectPasswordSucceeds(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()

	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: srv.config(),
		Net:    pluginNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if err := sess.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestConnectRejectsUnsupportedAgentAuth(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()
	cfg := srv.config()
	cfg["auth"] = "agent"

	_, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: cfg,
		Net:    pluginNet{},
	})
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("Connect error = %v, want ErrInvalidInput", err)
	}
}

func TestCredentialIdentityOverridesConnectionUser(t *testing.T) {
	opts, err := parseConnectOptions(plugin.ConnectConfig{Config: map[string]any{
		"host":                 "example.test",
		"user":                 "root",
		"auth":                 "credential",
		"_credential_secret":   "pw",
		"_credential_identity": "ubuntu",
	}})
	if err != nil {
		t.Fatalf("parseConnectOptions: %v", err)
	}
	if opts.User != "ubuntu" {
		t.Fatalf("user = %q, want credential identity", opts.User)
	}
	if opts.Password != "pw" || opts.PrivateKey != "pw" {
		t.Fatal("credential secret was not injected into auth material")
	}
}

type pluginNet struct{}

func (pluginNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (pluginNet) HTTP() (string, http.RoundTripper, bool) {
	return "", nil, false
}
