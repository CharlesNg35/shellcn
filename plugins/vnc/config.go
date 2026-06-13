package vnc

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const defaultPort = 5900

// CredentialVNCPassword is the plugin-owned reusable credential kind: a bare VNC
// password with no associated identity.
const CredentialVNCPassword plugin.CredentialKind = "vnc_password"

func credentialKinds() []plugin.CredentialKindInfo {
	return []plugin.CredentialKindInfo{
		{Kind: CredentialVNCPassword, Label: "VNC password", SecretLabel: "Password"},
	}
}

func configSchema(protocol string) plugin.Schema {
	hostValidators := []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[^\s/]+$`, Message: "Enter a host name or IP address, not a URL."}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host name or IP", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1", Validators: hostValidators},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
		}},
		{Name: "Auth", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
				{Label: "Password", Value: "password"},
				{Label: "Stored password", Value: "credential"},
				{Label: "None", Value: "none"},
			}},
			{Key: "credential_id", Label: "Stored password", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
				Kind: CredentialVNCPassword, Protocols: []string{protocol}, Required: true,
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		}},
	}}
}

type connectOptions struct {
	Host     string
	Port     int
	Password string
}

func parseConnectOptions(cfg plugin.ConnectConfig) (connectOptions, error) {
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	auth := strings.TrimSpace(cfg.String("auth"))
	if auth == "" {
		auth = "password"
	}
	switch auth {
	case "password", "credential", "none":
	default:
		return connectOptions{}, fmt.Errorf("%w: unsupported authentication method", plugin.ErrInvalidInput)
	}
	opts := connectOptions{
		Host:     strings.TrimSpace(cfg.String("host")),
		Port:     port,
		Password: cfg.String("password"),
	}
	if opts.Host == "" {
		return connectOptions{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return connectOptions{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	if strings.ContainsAny(opts.Host, " \t\r\n/") {
		return connectOptions{}, fmt.Errorf("%w: host must be a host name or IP address, not a URL", plugin.ErrInvalidInput)
	}
	if secret := cfg.CredentialSecretFor(plugin.CredentialField); secret != "" && auth == "credential" {
		opts.Password = secret
	}
	if auth == "none" {
		opts.Password = ""
	} else if strings.TrimSpace(opts.Password) == "" {
		return connectOptions{}, fmt.Errorf("%w: password is required for the selected authentication method", plugin.ErrInvalidInput)
	}
	return opts, nil
}

func connect(cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseConnectOptions(cfg)
	if err != nil {
		return nil, err
	}
	return &Session{
		net:      cfg.Net,
		addr:     net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
		password: opts.Password,
	}, nil
}
