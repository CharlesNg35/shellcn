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
	hostValidators := []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[^\s/]+$`, Message: "Enter a host name or IP address, not a URL."}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host name or IP", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1", Validators: hostValidators},
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
				Kind: CredentialRDPPassword, Protocols: []string{protocol}, Required: true,
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
		}},
		{Name: "Display", Fields: []plugin.Field{
			{Key: "resolution", Label: "Desktop size", Type: plugin.FieldSelect, Default: "1280x800", Help: "Initial remote desktop size. The browser scales this fixed session to fit the panel.", Options: []plugin.Option{
				{Label: "1280 x 800", Value: "1280x800"},
				{Label: "1600 x 900", Value: "1600x900"},
				{Label: "1920 x 1080", Value: "1920x1080"},
				{Label: "2560 x 1440", Value: "2560x1440"},
				{Label: "3840 x 2160", Value: "3840x2160"},
				{Label: "1366 x 768", Value: "1366x768"},
				{Label: "1024 x 768", Value: "1024x768"},
			}},
		}},
	}}
}

type connectOptions struct {
	Host     string
	Port     int
	User     string
	Domain   string
	Password string
	Width    int
	Height   int
}

// parseResolution turns "1920x1080" into width/height, falling back to the
// default for an empty or malformed value.
func parseResolution(s string) (int, int) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(s)), "x", 2)
	if len(parts) != 2 {
		return defaultWidth, defaultHeight
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w < 640 || h < 480 || w > 8192 || h > 8192 {
		return defaultWidth, defaultHeight
	}
	return w, h
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
	case "password", "credential":
	default:
		return connectOptions{}, fmt.Errorf("%w: unsupported authentication method", plugin.ErrInvalidInput)
	}
	w, h := parseResolution(cfg.String("resolution"))
	opts := connectOptions{
		Host:     strings.TrimSpace(cfg.String("host")),
		Port:     port,
		User:     strings.TrimSpace(cfg.String("username")),
		Domain:   strings.TrimSpace(cfg.String("domain")),
		Password: cfg.String("password"),
		Width:    w,
		Height:   h,
	}
	if auth == "credential" {
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
	if strings.ContainsAny(opts.Host, " \t\r\n/") {
		return connectOptions{}, fmt.Errorf("%w: host must be a host name or IP address, not a URL", plugin.ErrInvalidInput)
	}
	if opts.User == "" {
		return connectOptions{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	if strings.TrimSpace(opts.Password) == "" {
		return connectOptions{}, fmt.Errorf("%w: password is required for the selected authentication method", plugin.ErrInvalidInput)
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
		width:    opts.Width,
		height:   opts.Height,
	}, nil
}
