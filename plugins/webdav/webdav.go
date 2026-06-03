// Package webdav implements the WebDAV filesystem plugin.
package webdav

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const protocolName = "webdav"

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "WebDAV",
		Description:         "File browser for WebDAV endpoints.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "cloud"},
		Category:            plugin.CategoryFiles,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSingle,
		Tabs: []plugin.Panel{filesystem.FilesTab(
			protocolName,
			filesystem.WithMove(protocolName),
			filesystem.WithCopy(protocolName),
			filesystem.WithArchive(protocolName),
		)},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return filesystem.Routes(protocolName, protocolName)
}

func (p *Plugin) Connect(_ context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	client := gowebdav.NewClient(opts.URL, opts.Username, opts.Password)
	client.SetTimeout(30 * time.Second)
	client.SetTransport(&http.Transport{
		DialContext: cfg.Net.DialContext,
		TLSClientConfig: &tls.Config{
			ServerName:         opts.ServerName,
			InsecureSkipVerify: !opts.VerifyTLS,
		},
	})
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("%w: webdav connect failed: %v", plugin.ErrUnauthorized, err)
	}
	return &Session{fs: &Client{client: client, root: opts.RootPath}}, nil
}

func configSchema() plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "url", Label: "URL", Type: plugin.FieldText, Required: true, Placeholder: "https://files.example.com/dav/"},
			{Key: "root_path", Label: "Root path", Type: plugin.FieldText, Default: "/", Placeholder: "/"},
			{Key: "verify_tls", Label: "Verify TLS certificate", Type: plugin.FieldToggle, Default: true},
		}},
		{Name: "Authentication", Fields: authFields()},
	}}
}

func authFields() []plugin.Field {
	return []plugin.Field{
		{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
			{Label: "Username & password", Value: "password"},
			{Label: "Stored WebDAV credential", Value: "credential"},
			{Label: "None", Value: "none"},
		}},
		{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "credential_id", Label: "WebDAV credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
			Kinds: []plugin.CredentialKind{plugin.CredentialBasicAuth}, Protocols: []string{protocolName}, Required: true,
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
	}
}

type options struct {
	URL        string
	ServerName string
	Auth       string
	Username   string
	Password   string
	RootPath   string
	VerifyTLS  bool
}

func parseOptions(cfg plugin.ConnectConfig) (options, error) {
	rawURL := strings.TrimSpace(cfg.String("url"))
	if rawURL == "" {
		return options{}, fmt.Errorf("%w: url is required", plugin.ErrInvalidInput)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return options{}, fmt.Errorf("%w: url must be absolute", plugin.ErrInvalidInput)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return options{}, fmt.Errorf("%w: url scheme must be http or https", plugin.ErrInvalidInput)
	}
	opts := options{
		URL:        rawURL,
		ServerName: u.Hostname(),
		Auth:       strings.TrimSpace(cfg.String("auth")),
		Username:   strings.TrimSpace(cfg.String("username")),
		Password:   cfg.String("password"),
		RootPath:   strings.TrimSpace(cfg.String("root_path")),
		VerifyTLS:  boolValue(cfg, "verify_tls", true),
	}
	if opts.Auth == "" {
		opts.Auth = "password"
	}
	switch opts.Auth {
	case "password":
	case "credential":
		if identity := cfg.CredentialIdentityFor(plugin.CredentialField); identity != "" {
			opts.Username = identity
		}
		if secret := cfg.CredentialSecretFor(plugin.CredentialField); secret != "" {
			opts.Password = secret
		}
	case "none":
		opts.Username = ""
		opts.Password = ""
	default:
		return options{}, fmt.Errorf("%w: unsupported authentication method %q", plugin.ErrInvalidInput, opts.Auth)
	}
	if opts.Auth != "none" && opts.Username == "" {
		return options{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	if opts.Auth != "none" && opts.Password == "" {
		return options{}, fmt.Errorf("%w: password is required", plugin.ErrInvalidInput)
	}
	if opts.RootPath == "" {
		opts.RootPath = "/"
	}
	return opts, nil
}

type Session struct {
	fs *Client
}

func (s *Session) Filesystem() (filesystem.Client, error) {
	return s.fs, nil
}

func (s *Session) HealthCheck(context.Context) error {
	_, err := s.fs.client.Stat(s.fs.root)
	return err
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	return nil
}

type Client struct {
	client *gowebdav.Client
	root   string
}

func (c *Client) Home(context.Context) (string, error) {
	return c.root, nil
}

func (c *Client) ReadDir(_ context.Context, p string) ([]os.FileInfo, error) {
	return c.client.ReadDir(p)
}

func (c *Client) Stat(_ context.Context, p string) (os.FileInfo, error) {
	return c.client.Stat(p)
}

func (c *Client) Open(_ context.Context, p string) (io.ReadCloser, error) {
	return c.client.ReadStream(p)
}

func (c *Client) OpenRange(_ context.Context, p string, offset, length int64) (io.ReadCloser, error) {
	return c.client.ReadStreamRange(p, offset, length)
}

func (c *Client) Write(_ context.Context, p string, r io.Reader) error {
	return c.client.WriteStream(p, r, 0o644)
}

func (c *Client) Mkdir(_ context.Context, p string) error {
	return c.client.Mkdir(p, 0o755)
}

func (c *Client) Rename(_ context.Context, from, to string) error {
	return c.client.Rename(from, to, true)
}

func (c *Client) Remove(_ context.Context, p string, _ bool) error {
	return c.client.Remove(p)
}

func (c *Client) Move(_ context.Context, src, dst string) error {
	return c.client.Rename(src, dst, true)
}

func (c *Client) Copy(_ context.Context, src, dst string) error {
	return c.client.Copy(src, dst, true)
}

func (c *Client) MapError(err error) error {
	if gowebdav.IsErrNotFound(err) {
		return plugin.ErrNotFound
	}
	return nil
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
