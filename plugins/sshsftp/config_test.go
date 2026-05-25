package sshsftp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestConnectUnknownHostKeyRequiresVerification(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()

	_, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: map[string]any{"host": srv.Host, "port": srv.Port, "user": "u", "auth": "password", "password": "p"},
		Net:    pluginNet{},
	})
	var verify *plugin.VerificationRequired
	if !errors.As(err, &verify) {
		t.Fatalf("Connect error = %v, want VerificationRequired", err)
	}
	if verify.Kind != "host_key" || verify.Data["knownHostsLine"] == "" {
		t.Fatalf("unexpected verification payload: %+v", verify)
	}
}

func TestConnectKnownHostKeySucceeds(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()
	known := knownhosts.Line([]string{knownhosts.Normalize(net.JoinHostPort(srv.Host, srv.Port))}, srv.PublicKey)
	cfg := srv.config()
	cfg["known_hosts"] = known

	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: cfg,
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

func TestConnectChangedHostKeyRequiresVerification(t *testing.T) {
	trusted := newSSHServer(t)
	trusted.Close()
	changed := newSSHServer(t)
	defer changed.Close()
	known := knownhosts.Line([]string{knownhosts.Normalize(net.JoinHostPort(changed.Host, changed.Port))}, trusted.PublicKey)
	cfg := changed.config()
	cfg["known_hosts"] = known

	_, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: cfg,
		Net:    pluginNet{},
	})
	var verify *plugin.VerificationRequired
	if !errors.As(err, &verify) {
		t.Fatalf("Connect error = %v, want VerificationRequired", err)
	}
	if verify.Data["changed"] != true {
		t.Fatalf("changed key not marked changed: %+v", verify.Data)
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
