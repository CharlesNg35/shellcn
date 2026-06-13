package sshsftp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"golang.org/x/crypto/ssh"
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

func TestConnectVerifiesPinnedHostKey(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()

	cfg := srv.config()
	cfg["host_key"] = ssh.FingerprintSHA256(srv.PublicKey)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: cfg,
		Net:    pluginNet{},
	})
	if err != nil {
		t.Fatalf("Connect with matching host key: %v", err)
	}
	_ = sess.Close()

	cfg = srv.config()
	cfg["host_key"] = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	_, err = Connect(context.Background(), plugin.ConnectConfig{
		Config: cfg,
		Net:    pluginNet{},
	})
	if !errors.Is(err, plugin.ErrUnauthorized) {
		t.Fatalf("Connect with mismatched host key error = %v, want ErrUnauthorized", err)
	}
}

func TestHostKeyCallbackParsesOpenSSHKeys(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()

	for name, hostKey := range map[string]string{
		"public key":  string(ssh.MarshalAuthorizedKey(srv.PublicKey)),
		"known hosts": srv.Host + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(srv.PublicKey))),
	} {
		t.Run(name, func(t *testing.T) {
			cb, err := hostKeyCallback(hostKey)
			if err != nil {
				t.Fatalf("hostKeyCallback: %v", err)
			}
			if err := cb(srv.Host, nil, srv.PublicKey); err != nil {
				t.Fatalf("callback rejected matching key: %v", err)
			}
		})
	}

	if _, err := hostKeyCallback("not-a-key"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("invalid host key error = %v, want ErrInvalidInput", err)
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
