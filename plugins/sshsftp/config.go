// Package sshsftp contains the shared SSH session and SFTP route implementation.
package sshsftp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

const (
	defaultPort = 22
)

type connectOptions struct {
	Host       string
	Port       int
	User       string
	Auth       string
	Password   string
	PrivateKey string
	Passphrase string
	KnownHosts string
}

// Connect opens one SSH client for either the SSH or SFTP plugin.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseConnectOptions(cfg)
	if err != nil {
		return nil, err
	}
	auth, closeAuth, err := authMethods(opts)
	if err != nil {
		return nil, err
	}
	defer closeAuth()

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	hostKey, cleanup, err := hostKeyCallback(opts, addr)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	conn, err := cfg.Net.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: dial ssh target: %v", plugin.ErrUnavailable, err)
	}
	sshCfg := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         15 * time.Second,
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		_ = conn.Close()
		var verify *plugin.VerificationRequired
		if errors.As(err, &verify) {
			return nil, verify
		}
		return nil, fmt.Errorf("%w: ssh handshake failed: %v", plugin.ErrUnauthorized, err)
	}
	return NewSession(ssh.NewClient(cc, chans, reqs)), nil
}

func parseConnectOptions(cfg plugin.ConnectConfig) (connectOptions, error) {
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	user := strings.TrimSpace(cfg.String("user"))
	if user == "" {
		user = strings.TrimSpace(cfg.String("username"))
	}
	opts := connectOptions{
		Host:       strings.TrimSpace(cfg.String("host")),
		Port:       port,
		User:       user,
		Auth:       strings.TrimSpace(cfg.String("auth")),
		Password:   cfg.String("password"),
		PrivateKey: cfg.String("private_key"),
		Passphrase: cfg.String("passphrase"),
		KnownHosts: cfg.String("known_hosts"),
	}
	if opts.Auth == "" {
		opts.Auth = "password"
	}
	if opts.Host == "" {
		return connectOptions{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return connectOptions{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	if opts.User == "" {
		return connectOptions{}, fmt.Errorf("%w: user is required", plugin.ErrInvalidInput)
	}
	if secret := cfg.String(service.CredentialSecret); secret != "" {
		if opts.Auth == "credential" {
			opts.Password = secret
			opts.PrivateKey = secret
		}
	}
	return opts, nil
}

func authMethods(opts connectOptions) ([]ssh.AuthMethod, func(), error) {
	switch opts.Auth {
	case "password":
		if opts.Password == "" {
			return nil, func() {}, fmt.Errorf("%w: password is required", plugin.ErrInvalidInput)
		}
		return []ssh.AuthMethod{ssh.Password(opts.Password)}, func() {}, nil
	case "private_key":
		method, err := privateKeyAuth(opts.PrivateKey, opts.Passphrase)
		return method, func() {}, err
	case "credential":
		if method, err := privateKeyAuth(opts.PrivateKey, opts.Passphrase); err == nil {
			return method, func() {}, nil
		}
		if opts.Password == "" {
			return nil, func() {}, fmt.Errorf("%w: credential secret is required", plugin.ErrInvalidInput)
		}
		return []ssh.AuthMethod{ssh.Password(opts.Password)}, func() {}, nil
	case "agent":
		socket := os.Getenv("SSH_AUTH_SOCK")
		if socket == "" {
			return nil, func() {}, fmt.Errorf("%w: SSH_AUTH_SOCK is not set", plugin.ErrUnavailable)
		}
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return nil, func() {}, fmt.Errorf("%w: connect ssh-agent: %v", plugin.ErrUnavailable, err)
		}
		client := agent.NewClient(conn)
		return []ssh.AuthMethod{ssh.PublicKeysCallback(client.Signers)}, func() { _ = conn.Close() }, nil
	default:
		return nil, func() {}, fmt.Errorf("%w: unsupported auth method %q", plugin.ErrInvalidInput, opts.Auth)
	}
}

func privateKeyAuth(pem, passphrase string) ([]ssh.AuthMethod, error) {
	if strings.TrimSpace(pem) == "" {
		return nil, fmt.Errorf("%w: private key is required", plugin.ErrInvalidInput)
	}
	var (
		signer ssh.Signer
		err    error
	)
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(pem), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(pem))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: parse private key: %v", plugin.ErrInvalidInput, err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func hostKeyCallback(opts connectOptions, addr string) (ssh.HostKeyCallback, func(), error) {
	if strings.TrimSpace(opts.KnownHosts) == "" {
		return challengeCallback(opts, addr, nil), func() {}, nil
	}
	f, err := os.CreateTemp("", "shellcn-known-hosts-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("%w: known_hosts temp file: %v", plugin.ErrUnavailable, err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(opts.KnownHosts); err != nil {
		_ = f.Close()
		cleanup()
		return nil, func() {}, fmt.Errorf("%w: known_hosts temp file: %v", plugin.ErrUnavailable, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("%w: known_hosts temp file: %v", plugin.ErrUnavailable, err)
	}
	cb, err := knownhosts.New(f.Name())
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("%w: parse known_hosts: %v", plugin.ErrInvalidInput, err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := cb(hostname, remote, key); err != nil {
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) {
				return hostKeyChallenge(opts, addr, key, keyErr)
			}
			return err
		}
		return nil
	}, cleanup, nil
}

func challengeCallback(opts connectOptions, addr string, keyErr *knownhosts.KeyError) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		return hostKeyChallenge(opts, addr, key, keyErr)
	}
}

func hostKeyChallenge(opts connectOptions, addr string, key ssh.PublicKey, keyErr *knownhosts.KeyError) error {
	changed := keyErr != nil && len(keyErr.Want) > 0
	message := "Unknown SSH host key"
	if changed {
		message = "SSH host key changed"
	}
	accepted := make([]string, 0)
	if keyErr != nil {
		for _, want := range keyErr.Want {
			accepted = append(accepted, ssh.FingerprintSHA256(want.Key))
		}
	}
	line := knownhosts.Line(hostKeyAddresses(opts, addr), key)
	return &plugin.VerificationRequired{
		Kind:    "host_key",
		Message: message,
		Data: map[string]any{
			"host":           opts.Host,
			"port":           opts.Port,
			"algorithm":      key.Type(),
			"fingerprint":    ssh.FingerprintSHA256(key),
			"knownHostsLine": line,
			"changed":        changed,
			"accepted":       accepted,
		},
	}
}

func hostKeyAddresses(opts connectOptions, addr string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(knownhosts.Normalize(addr))
	if opts.Port == defaultPort {
		add(knownhosts.Normalize(opts.Host))
	}
	return out
}
