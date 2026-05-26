// Package ftpfs adapts github.com/jlaffaye/ftp to ShellCN's shared file browser.
package ftpfs

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	ftplib "github.com/jlaffaye/ftp"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/plugins/shared/filesystem"
)

const (
	DefaultFTPPort  = 21
	DefaultFTPSPort = 990
)

type TLSMode string

const (
	TLSNone     TLSMode = "none"
	TLSExplicit TLSMode = "explicit"
	TLSImplicit TLSMode = "implicit"
)

type Options struct {
	Host      string
	Port      int
	Auth      string
	Username  string
	Password  string
	RootPath  string
	TLSMode   TLSMode
	VerifyTLS bool
}

type Session struct {
	client *Client
}

func NewSession(client *Client) *Session {
	return &Session{client: client}
}

func (s *Session) Filesystem() (filesystem.Client, error) {
	return s.client, nil
}

func (s *Session) HealthCheck(context.Context) error {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	return s.client.conn.NoOp()
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	return s.client.conn.Quit()
}

type Client struct {
	mu   sync.Mutex
	conn *ftplib.ServerConn
	root string
}

func Connect(ctx context.Context, cfg plugin.ConnectConfig, opts Options) (plugin.Session, error) {
	if err := normalizeOptions(cfg, &opts); err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	dial := func(network, address string) (net.Conn, error) {
		return cfg.Net.DialContext(ctx, network, address)
	}
	ftpOpts := []ftplib.DialOption{
		ftplib.DialWithContext(ctx),
		ftplib.DialWithDialFunc(dial),
		ftplib.DialWithTimeout(15 * time.Second),
		ftplib.DialWithShutTimeout(10 * time.Second),
	}
	if opts.TLSMode != TLSNone {
		tlsConfig := &tls.Config{ServerName: opts.Host, InsecureSkipVerify: !opts.VerifyTLS}
		if opts.TLSMode == TLSImplicit {
			ftpOpts = append(ftpOpts, ftplib.DialWithTLS(tlsConfig))
		} else {
			ftpOpts = append(ftpOpts, ftplib.DialWithExplicitTLS(tlsConfig))
		}
	}
	conn, err := ftplib.Dial(addr, ftpOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: dial ftp target: %v", plugin.ErrUnavailable, err)
	}
	if err := conn.Login(opts.Username, opts.Password); err != nil {
		_ = conn.Quit()
		return nil, fmt.Errorf("%w: ftp login failed: %v", plugin.ErrUnauthorized, err)
	}
	return NewSession(&Client{conn: conn, root: opts.RootPath}), nil
}

func normalizeOptions(cfg plugin.ConnectConfig, opts *Options) error {
	opts.Host = strings.TrimSpace(opts.Host)
	if opts.Host == "" {
		opts.Host = strings.TrimSpace(cfg.String("host"))
	}
	if opts.Host == "" {
		return fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	if opts.Port == 0 {
		if port, ok := cfg.Int("port"); ok {
			opts.Port = port
		}
	}
	if opts.Port == 0 {
		opts.Port = DefaultFTPPort
		if opts.TLSMode == TLSImplicit {
			opts.Port = DefaultFTPSPort
		}
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	opts.Auth = strings.TrimSpace(cfg.String("auth"))
	if opts.Auth == "" {
		opts.Auth = "password"
	}
	opts.Username = strings.TrimSpace(cfg.String("username"))
	opts.Password = cfg.String("password")
	switch opts.Auth {
	case "password":
	case "credential":
		if identity := strings.TrimSpace(cfg.String(service.CredentialIdentity)); identity != "" {
			opts.Username = identity
		}
		if secret := cfg.String(service.CredentialSecret); secret != "" {
			opts.Password = secret
		}
	case "anonymous":
		opts.Username = "anonymous"
		opts.Password = "anonymous@"
	default:
		return fmt.Errorf("%w: unsupported authentication method %q", plugin.ErrInvalidInput, opts.Auth)
	}
	if opts.Username == "" {
		return fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	if opts.Auth != "anonymous" && opts.Password == "" {
		return fmt.Errorf("%w: password is required", plugin.ErrInvalidInput)
	}
	opts.RootPath = strings.TrimSpace(cfg.String("root_path"))
	if opts.RootPath == "" {
		opts.RootPath = "/"
	}
	opts.VerifyTLS = boolValue(cfg, "verify_tls", opts.VerifyTLS)
	return nil
}

func (c *Client) Home(context.Context) (string, error) {
	return c.root, nil
}

func (c *Client) ReadDir(_ context.Context, p string) ([]os.FileInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := c.conn.List(p)
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		infos = append(infos, ftpInfo{entry: entry})
	}
	return infos, nil
}

func (c *Client) Stat(_ context.Context, p string) (os.FileInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, err := c.conn.GetEntry(p)
	if err == nil {
		entry.Name = path.Base(p)
		return ftpInfo{entry: entry}, nil
	}
	parent := path.Dir(p)
	base := path.Base(p)
	entries, listErr := c.conn.List(parent)
	if listErr != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name == base {
			return ftpInfo{entry: entry}, nil
		}
	}
	return nil, os.ErrNotExist
}

func (c *Client) Open(_ context.Context, p string) (io.ReadCloser, error) {
	c.mu.Lock()
	r, err := c.conn.Retr(p)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	return &lockedReadCloser{ReadCloser: r, unlock: c.mu.Unlock}, nil
}

func (c *Client) Write(_ context.Context, p string, r io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Stor(p, r)
}

func (c *Client) Mkdir(_ context.Context, p string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.MakeDir(p)
}

func (c *Client) Rename(_ context.Context, from, to string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Rename(from, to)
}

func (c *Client) Remove(_ context.Context, p string, isDir bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if isDir {
		return c.conn.RemoveDir(p)
	}
	return c.conn.Delete(p)
}

func (c *Client) MapError(err error) error {
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		switch protoErr.Code {
		case 530:
			return plugin.ErrUnauthorized
		case 550:
			return plugin.ErrNotFound
		case 553:
			return plugin.ErrInvalidInput
		}
	}
	return nil
}

type ftpInfo struct {
	entry *ftplib.Entry
}

func (i ftpInfo) Name() string {
	return i.entry.Name
}

func (i ftpInfo) Size() int64 {
	return int64(i.entry.Size)
}

func (i ftpInfo) Mode() os.FileMode {
	switch i.entry.Type {
	case ftplib.EntryTypeFolder:
		return os.ModeDir | 0o755
	case ftplib.EntryTypeLink:
		return os.ModeSymlink | 0o777
	default:
		return 0o644
	}
}

func (i ftpInfo) ModTime() time.Time {
	return i.entry.Time
}

func (i ftpInfo) IsDir() bool {
	return i.entry.Type == ftplib.EntryTypeFolder
}

func (i ftpInfo) Sys() any {
	return i.entry
}

type lockedReadCloser struct {
	io.ReadCloser
	once   sync.Once
	unlock func()
}

func (r *lockedReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.once.Do(r.unlock)
	return err
}

func boolValue(cfg plugin.ConnectConfig, key string, fallback bool) bool {
	switch v := cfg.Config[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return fallback
	}
}
