package proxmox

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const defaultPort = 8006

// CredentialProxmoxToken is the plugin-owned reusable credential kind: a Proxmox
// API token (identity is the token id `user@realm!name`, secret is the value).
const CredentialProxmoxToken plugin.CredentialKind = "proxmox_api_token"

func credentialKinds() []plugin.CredentialKindInfo {
	return []plugin.CredentialKindInfo{
		{
			Kind: CredentialProxmoxToken, Label: "Proxmox API token",
			Fields: []plugin.Field{
				plugin.CredentialPublicField(plugin.Field{Key: "token_id", Label: "Token ID (user@realm!name)", Type: plugin.FieldText, Required: true}),
				plugin.CredentialSecretField(plugin.Field{Key: "token_secret", Label: "Token secret", Type: plugin.FieldPassword, Required: true}),
			},
		},
	}
}

func configSchema(protocol string) plugin.Schema {
	whenToken := &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "token"}}}
	whenPassword := &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}
	whenCredential := &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "pve.example.com"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "verify_tls", Label: "Verify TLS certificate", Type: plugin.FieldToggle, Default: false, Help: "Disable for the self-signed certificate a default Proxmox install ships with."},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Method", Type: plugin.FieldSelect, Required: true, Default: "token", Options: []plugin.Option{
				{Label: "API token", Value: "token"},
				{Label: "Username & password", Value: "password"},
				{Label: "Stored API token", Value: "credential"},
			}},
			{Key: "token_id", Label: "Token ID", Type: plugin.FieldText, Required: true, Placeholder: "root@pam!shellcn", VisibleWhen: whenToken},
			{Key: "token_secret", Label: "Token secret", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: whenToken},
			{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, Placeholder: "root@pam", VisibleWhen: whenPassword},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: whenPassword},
			{Key: "credential_id", Label: "API token credential", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: CredentialProxmoxToken, Protocols: []string{protocol},
			}, VisibleWhen: whenCredential},
		}},
	}}
}

type authMethod string

const (
	authToken    authMethod = "token"
	authPassword authMethod = "password"
)

type connectOptions struct {
	Addr        string // host:port
	VerifyTLS   bool
	Method      authMethod
	TokenID     string
	TokenSecret string
	Username    string
	Password    string
}

func parseConnectOptions(cfg plugin.ConnectConfig) (connectOptions, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return connectOptions{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return connectOptions{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}

	opts := connectOptions{
		Addr:      net.JoinHostPort(host, strconv.Itoa(port)),
		VerifyTLS: boolValue(cfg, "verify_tls", false),
	}

	switch strings.TrimSpace(cfg.String("auth")) {
	case "", "token":
		opts.Method = authToken
		opts.TokenID = strings.TrimSpace(cfg.String("token_id"))
		opts.TokenSecret = cfg.String("token_secret")
	case "password":
		opts.Method = authPassword
		opts.Username = strings.TrimSpace(cfg.String("username"))
		opts.Password = cfg.String("password")
	case "credential":
		opts.Method = authToken
		opts.TokenID = cfg.CredentialValueFor(plugin.CredentialRefField, "token_id")
		opts.TokenSecret = cfg.CredentialValueFor(plugin.CredentialRefField, "token_secret")
	default:
		return connectOptions{}, fmt.Errorf("%w: unsupported authentication method", plugin.ErrInvalidInput)
	}

	switch opts.Method {
	case authToken:
		if opts.TokenID == "" || opts.TokenSecret == "" {
			return connectOptions{}, fmt.Errorf("%w: API token id and secret are required", plugin.ErrInvalidInput)
		}
	case authPassword:
		if opts.Username == "" || opts.Password == "" {
			return connectOptions{}, fmt.Errorf("%w: username and password are required", plugin.ErrInvalidInput)
		}
	}
	return opts, nil
}

func boolValue(cfg plugin.ConnectConfig, key string, def bool) bool {
	if v, ok := cfg.Config[key].(bool); ok {
		return v
	}
	return def
}
