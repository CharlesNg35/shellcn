// Package sshsftp contains the shared SSH session and SFTP route implementation.
package sshsftp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

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
}

// Connect opens one SSH client for either the SSH or SFTP plugin.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseConnectOptions(cfg)
	if err != nil {
		return nil, err
	}
	auth, err := authMethods(opts)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	conn, err := cfg.Net.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: dial ssh target: %v", plugin.ErrUnavailable, err)
	}
	sshCfg := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		_ = conn.Close()
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

func authMethods(opts connectOptions) ([]ssh.AuthMethod, error) {
	switch opts.Auth {
	case "password":
		if opts.Password == "" {
			return nil, fmt.Errorf("%w: password is required", plugin.ErrInvalidInput)
		}
		return []ssh.AuthMethod{ssh.Password(opts.Password)}, nil
	case "private_key":
		return privateKeyAuth(opts.PrivateKey, opts.Passphrase)
	case "credential":
		if method, err := privateKeyAuth(opts.PrivateKey, opts.Passphrase); err == nil {
			return method, nil
		}
		if opts.Password == "" {
			return nil, fmt.Errorf("%w: credential secret is required", plugin.ErrInvalidInput)
		}
		return []ssh.AuthMethod{ssh.Password(opts.Password)}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported auth method %q", plugin.ErrInvalidInput, opts.Auth)
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
