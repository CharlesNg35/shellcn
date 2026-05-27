package ldap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/dbcred"
)

const (
	protocolName    = "ldap"
	defaultPort     = 389
	defaultTimeout  = 10 * time.Second
	defaultSize     = 200
	defaultPageSize = 100

	clientCertField   = "client_cert_id"
	credentialIDField = "credential_id"

	authAnonymous  = "anonymous"
	authSimple     = "simple"
	authCredential = "credential"

	encNone     = "none"
	encStartTLS = "starttls"
	encLDAPS    = "ldaps"
)

type options struct {
	Host              string
	Port              int
	BaseDN            string
	Encryption        string
	SkipVerify        bool
	CACertificate     string
	ClientCertificate string
	AuthMode          string
	BindDN            string
	Password          string
	ReadOnly          bool
	Timeout           time.Duration
	SizeLimit         int
	PageSize          int
}

func configSchema() plugin.Schema {
	simpleAuth := plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authSimple}, {Field: credentialIDField, Op: plugin.OpEmpty}}}
	credentialAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authCredential}, {Field: credentialIDField, Op: plugin.OpNotEmpty}}}
	encrypted := plugin.Condition{AllOf: []plugin.Rule{{Field: "encryption", Op: plugin.OpNeq, Value: encNone}}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "ldap.example.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "base_dn", Label: "Base DN", Type: plugin.FieldText, Placeholder: "dc=example,dc=com", Help: "Directory root to browse. Leave empty to auto-detect from the server's root DSE."},
		}},
		{Name: "Encryption", Fields: []plugin.Field{
			{Key: "encryption", Label: "Encryption", Type: plugin.FieldSelect, Required: true, Default: encNone, Options: []plugin.Option{
				{Label: "None (plain)", Value: encNone},
				{Label: "StartTLS", Value: encStartTLS},
				{Label: "LDAPS", Value: encLDAPS},
			}},
			{Key: "tls_skip_verify", Label: "Skip certificate verification", Type: plugin.FieldToggle, VisibleWhen: &encrypted, Help: "Accept any server certificate. Use only for trusted networks or self-signed test servers."},
			{Key: "ca_certificate", Label: "CA certificate", Type: plugin.FieldTextarea, Secret: true, VisibleWhen: &encrypted, Help: "PEM CA bundle used to verify the server certificate."},
			{Key: clientCertField, Label: "Client certificate", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialTLSClientCert}, Protocols: []string{protocolName},
			}, VisibleWhen: &encrypted, Help: "Optional PEM containing the client certificate and private key for mutual TLS."},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: authAnonymous, Options: []plugin.Option{
				{Label: "Anonymous", Value: authAnonymous},
				{Label: "Simple bind", Value: authSimple},
				{Label: "Stored credential", Value: authCredential},
			}},
			{Key: "bind_dn", Label: "Bind DN", Type: plugin.FieldText, Placeholder: "cn=admin,dc=example,dc=com", VisibleWhen: &simpleAuth},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &simpleAuth},
			{Key: credentialIDField, Label: "Stored credential", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialBasicAuth}, Protocols: []string{protocolName},
			}, VisibleWhen: &credentialAuth, Help: "Reusable bind credential. Its identity is the bind DN and its secret is the password."},
		}},
		{Name: "Safety", Fields: []plugin.Field{
			{Key: "read_only", Label: "Read-only mode", Type: plugin.FieldToggle, Default: true, Help: "Blocks add, modify, delete, and rename operations when enabled."},
			{Key: "timeout", Label: "Operation timeout", Type: plugin.FieldDuration, Default: defaultTimeout.String()},
			{Key: "size_limit", Label: "Search size limit", Type: plugin.FieldNumber, Default: defaultSize, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
			{Key: "page_size", Label: "Page size", Type: plugin.FieldNumber, Default: defaultPageSize, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
		}},
	}}
}

func parseOptions(cfg plugin.ConnectConfig) (options, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return options{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return options{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	encryption := stringDefault(cfg.String("encryption"), encNone)
	switch encryption {
	case encNone, encStartTLS, encLDAPS:
	default:
		return options{}, fmt.Errorf("%w: unsupported encryption %q", plugin.ErrInvalidInput, encryption)
	}

	authMode := stringDefault(cfg.String("auth"), authAnonymous)
	var bindDN, password string
	switch authMode {
	case authAnonymous:
	case authSimple, authCredential:
		material := dbcred.ApplyPasswordCredential(cfg, cfg.String("bind_dn"), cfg.String("password"))
		bindDN, password = material.Username, material.Password
		if strings.TrimSpace(bindDN) == "" {
			return options{}, fmt.Errorf("%w: bind DN is required for authenticated binds", plugin.ErrInvalidInput)
		}
	default:
		return options{}, fmt.Errorf("%w: unsupported authentication method", plugin.ErrInvalidInput)
	}

	sizeLimit := intValue(cfg.Config["size_limit"], defaultSize)
	if sizeLimit > plugin.MaxPageLimit {
		sizeLimit = plugin.MaxPageLimit
	}
	pageSize := intValue(cfg.Config["page_size"], defaultPageSize)
	if pageSize > plugin.MaxPageLimit {
		pageSize = plugin.MaxPageLimit
	}
	return options{
		Host:              host,
		Port:              port,
		BaseDN:            strings.TrimSpace(cfg.String("base_dn")),
		Encryption:        encryption,
		SkipVerify:        boolValue(cfg.Config["tls_skip_verify"], false),
		CACertificate:     cfg.String("ca_certificate"),
		ClientCertificate: dbcred.ResolvedSecret(cfg, clientCertField),
		AuthMode:          authMode,
		BindDN:            bindDN,
		Password:          password,
		ReadOnly:          boolValue(cfg.Config["read_only"], true),
		Timeout:           durationValue(cfg.Config["timeout"], defaultTimeout),
		SizeLimit:         sizeLimit,
		PageSize:          pageSize,
	}, nil
}

func (o options) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: o.Host}
	if o.SkipVerify {
		cfg.InsecureSkipVerify = true //nolint:gosec // explicit opt-in for self-signed/test directories.
	}
	if o.CACertificate != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(o.CACertificate)) {
			return nil, fmt.Errorf("%w: CA certificate is not valid PEM", plugin.ErrInvalidInput)
		}
		cfg.RootCAs = pool
	}
	if o.ClientCertificate != "" {
		cert, err := tls.X509KeyPair([]byte(o.ClientCertificate), []byte(o.ClientCertificate))
		if err != nil {
			return nil, fmt.Errorf("%w: client certificate credential must contain certificate and private key PEM", plugin.ErrInvalidInput)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

func boolValue(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func intValue(v any, def int) int {
	switch n := v.(type) {
	case int:
		if n > 0 {
			return n
		}
	case int64:
		if n > 0 {
			return int(n)
		}
	case float64:
		if n > 0 {
			return int(n)
		}
	}
	return def
}

func durationValue(v any, def time.Duration) time.Duration {
	switch t := v.(type) {
	case string:
		if d, err := time.ParseDuration(strings.TrimSpace(t)); err == nil && d > 0 {
			return d
		}
	case float64:
		if t > 0 {
			return time.Duration(t) * time.Second
		}
	case int:
		if t > 0 {
			return time.Duration(t) * time.Second
		}
	}
	return def
}

func stringDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
