// Package smb implements the SMB filesystem plugin.
package smb

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/hirochachacha/go-smb2"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	protocolName = "smb"
	defaultPort  = 445
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "SMB",
		Description:         "File browser for SMB/CIFS shares.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "hard-drive"},
		Category:            plugin.CategoryFiles,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSingle,
		Tabs: []plugin.Panel{filesystem.FilesTab(
			protocolName,
			filesystem.WithMove(protocolName),
			filesystem.WithCopy(protocolName),
			filesystem.WithChmod(protocolName),
			filesystem.WithArchive(protocolName),
		)},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return filesystem.Routes(protocolName, protocolName)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := cfg.Net.DialContext(ctx, "tcp", net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)))
	if err != nil {
		return nil, fmt.Errorf("%w: dial smb target: %v", plugin.ErrUnavailable, err)
	}
	session, err := (&smb2.Dialer{Initiator: &smb2.NTLMInitiator{
		User:     opts.Username,
		Password: opts.Password,
		Domain:   opts.Domain,
	}}).DialContext(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: smb login failed: %v", plugin.ErrUnauthorized, err)
	}
	share, err := session.Mount(opts.Share)
	if err != nil {
		_ = session.Logoff()
		_ = conn.Close()
		return nil, fmt.Errorf("%w: smb mount failed: %v", plugin.ErrUnavailable, err)
	}
	return &Session{conn: conn, session: session, fs: &Client{share: share, root: opts.RootPath}}, nil
}

func configSchema() plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "fileserver.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "share", Label: "Share", Type: plugin.FieldText, Required: true, Placeholder: "shared"},
			{Key: "root_path", Label: "Root path", Type: plugin.FieldText, Default: "/", Placeholder: "/"},
			{Key: "domain", Label: "Domain", Type: plugin.FieldText, Placeholder: "WORKGROUP"},
		}},
		{Name: "Authentication", Fields: authFields()},
	}}
}

func authFields() []plugin.Field {
	return []plugin.Field{
		{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
			{Label: "Username & password", Value: "password"},
			{Label: "Stored SMB credential", Value: "credential"},
			{Label: "Guest", Value: "guest"},
		}},
		{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "credential_id", Label: "SMB credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
			Kind: plugin.CredentialBasicAuth, Protocols: []string{protocolName}, Required: true,
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
	}
}

type options struct {
	Host     string
	Port     int
	Share    string
	RootPath string
	Domain   string
	Auth     string
	Username string
	Password string
}

func parseOptions(cfg plugin.ConnectConfig) (options, error) {
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	opts := options{
		Host:     strings.TrimSpace(cfg.String("host")),
		Port:     port,
		Share:    strings.Trim(strings.TrimSpace(cfg.String("share")), `/\`),
		RootPath: normalizeRootPath(cfg.String("root_path")),
		Domain:   strings.TrimSpace(cfg.String("domain")),
		Auth:     strings.TrimSpace(cfg.String("auth")),
		Username: strings.TrimSpace(cfg.String("username")),
		Password: cfg.String("password"),
	}
	if opts.Host == "" {
		return options{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return options{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	if opts.Share == "" {
		return options{}, fmt.Errorf("%w: share is required", plugin.ErrInvalidInput)
	}
	if opts.Auth == "" {
		opts.Auth = "password"
	}
	switch opts.Auth {
	case "password":
	case "credential":
		if identity := cfg.CredentialValueFor(plugin.CredentialIDField, "username"); identity != "" {
			opts.Username = identity
		}
		if secret := cfg.CredentialValueFor(plugin.CredentialIDField, "password"); secret != "" {
			opts.Password = secret
		}
	case "guest":
		opts.Username = ""
		opts.Password = ""
	default:
		return options{}, fmt.Errorf("%w: unsupported authentication method %q", plugin.ErrInvalidInput, opts.Auth)
	}
	if opts.Auth != "guest" && opts.Username == "" {
		return options{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	if opts.Auth != "guest" && opts.Password == "" {
		return options{}, fmt.Errorf("%w: password is required", plugin.ErrInvalidInput)
	}
	return opts, nil
}

func normalizeRootPath(raw string) string {
	p := strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	if p == "" || p == "." {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

type Session struct {
	conn    net.Conn
	session *smb2.Session
	fs      *Client
}

func (s *Session) Filesystem() (filesystem.Client, error) {
	return s.fs, nil
}

func (s *Session) HealthCheck(context.Context) error {
	_, err := s.fs.share.Stat(smbPath(s.fs.root))
	return err
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	var err error
	if s.fs != nil && s.fs.share != nil {
		err = s.fs.share.Umount()
	}
	if s.session != nil {
		if logoffErr := s.session.Logoff(); err == nil {
			err = logoffErr
		}
	}
	if s.conn != nil {
		if closeErr := s.conn.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

type Client struct {
	share *smb2.Share
	root  string
}

func (c *Client) Home(context.Context) (string, error) {
	return c.root, nil
}

func (c *Client) ReadDir(_ context.Context, p string) ([]os.FileInfo, error) {
	return c.share.ReadDir(smbPath(p))
}

func (c *Client) Stat(_ context.Context, p string) (os.FileInfo, error) {
	return c.share.Stat(smbPath(p))
}

func (c *Client) Open(_ context.Context, p string) (io.ReadCloser, error) {
	return c.share.Open(smbPath(p))
}

func (c *Client) OpenSeeker(_ context.Context, p string) (io.ReadSeekCloser, error) {
	f, err := c.share.Open(smbPath(p))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (c *Client) Write(_ context.Context, p string, r io.Reader) error {
	f, err := c.share.OpenFile(smbPath(p), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *Client) Mkdir(_ context.Context, p string) error {
	return c.share.Mkdir(smbPath(p), 0o755)
}

func (c *Client) Rename(_ context.Context, from, to string) error {
	return c.share.Rename(smbPath(from), smbPath(to))
}

func (c *Client) Remove(_ context.Context, p string, _ bool) error {
	return c.share.Remove(smbPath(p))
}

func (c *Client) Move(_ context.Context, src, dst string) error {
	return c.share.Rename(smbPath(src), smbPath(dst))
}

func (c *Client) Copy(ctx context.Context, src, dst string) error {
	r, err := c.Open(ctx, src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	return c.Write(ctx, dst, r)
}

func (c *Client) Chmod(_ context.Context, p string, mode os.FileMode) error {
	return c.share.Chmod(smbPath(p), mode)
}

func smbPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "."
	}
	return p
}
