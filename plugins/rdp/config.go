package rdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	defaultPort   = 3389
	defaultWidth  = 1280
	defaultHeight = 800
)

// CredentialRDPPassword is the plugin-owned reusable credential kind: an RDP
// username (identity) plus password (secret).
const CredentialRDPPassword plugin.CredentialKind = "rdp_password"

func credentialKinds() []plugin.CredentialKindInfo {
	return []plugin.CredentialKindInfo{
		{Kind: CredentialRDPPassword, Label: "RDP password", SecretLabel: "Password", IdentityLabel: "Username"},
	}
}

func configSchema(protocol string) plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "domain", Label: "Domain", Type: plugin.FieldText, Placeholder: "WORKGROUP"},
		}},
		{Name: "Auth", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
				{Label: "Password", Value: "password"},
				{Label: "Stored password", Value: "credential"},
			}},
			{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
			{Key: "credential_id", Label: "Stored password", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{CredentialRDPPassword}, Protocols: []string{protocol}, Required: true,
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
		}},
	}}
}

type connectOptions struct {
	Host     string
	Port     int
	User     string
	Domain   string
	Password string
}

func parseConnectOptions(cfg plugin.ConnectConfig) (connectOptions, error) {
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	opts := connectOptions{
		Host:     strings.TrimSpace(cfg.String("host")),
		Port:     port,
		User:     strings.TrimSpace(cfg.String("username")),
		Domain:   strings.TrimSpace(cfg.String("domain")),
		Password: cfg.String("password"),
	}
	if strings.TrimSpace(cfg.String("auth")) == "credential" {
		if secret := cfg.CredentialSecretFor(plugin.CredentialField); secret != "" {
			opts.Password = secret
		}
		if identity := cfg.CredentialIdentityFor(plugin.CredentialField); identity != "" {
			opts.User = identity
		}
	}
	if opts.Host == "" {
		return connectOptions{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return connectOptions{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	if opts.User == "" {
		return connectOptions{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	return opts, nil
}

func connect(cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseConnectOptions(cfg)
	if err != nil {
		return nil, err
	}
	user := opts.User
	if opts.Domain != "" {
		user = opts.Domain + "\\" + opts.User
	}
	return &Session{
		addr:     net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
		user:     user,
		password: opts.Password,
		width:    defaultWidth,
		height:   defaultHeight,
	}, nil
}
