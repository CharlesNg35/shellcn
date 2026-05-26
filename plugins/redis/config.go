package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

const (
	protocolName      = "redis"
	defaultPort       = 6379
	defaultTimeout    = 5 * time.Second
	defaultPoolSize   = 8
	defaultScanCount  = 200
	defaultValueLimit = 500
	credentialIDField = "credential_id"
	clientCertField   = "client_cert_id"
	authPassword      = "password"
	authCredential    = "credential"
)

type options struct {
	Host              string
	Port              int
	Database          int
	Username          string
	Password          string
	TLSMode           string
	CACertificate     string
	ClientCertificate string
	ReadOnly          bool
	RequireConfirm    bool
	Timeout           time.Duration
	PoolSize          int
	ScanCount         int
	ValueLimit        int
	KeyPattern        string
}

func configSchema() plugin.Schema {
	passwordAuth := plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authPassword}, {Field: credentialIDField, Op: plugin.OpEmpty}}}
	credentialAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authCredential}, {Field: credentialIDField, Op: plugin.OpNotEmpty}}}
	tlsEnabled := plugin.Condition{AllOf: []plugin.Rule{{Field: "tls_mode", Op: plugin.OpNeq, Value: "disable"}}}
	verifyTLS := plugin.Condition{AnyOf: []plugin.Rule{
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-ca"},
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-full"},
	}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "redis.example.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "database", Label: "Database", Type: plugin.FieldNumber, Default: 0, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}, {Type: plugin.ValidatorMax, Value: 15}}},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: authPassword, Options: []plugin.Option{
				{Label: "Password", Value: authPassword},
				{Label: "Stored credential", Value: authCredential},
			}},
			{Key: "username", Label: "Username", Type: plugin.FieldText, Placeholder: "default", VisibleWhen: &passwordAuth},
			{Key: credentialIDField, Label: "Stored password", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialDBPassword}, Protocols: []string{protocolName},
			}, VisibleWhen: &credentialAuth, Help: "Reusable Redis password. The credential identity can also supply the ACL username."},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &passwordAuth},
		}},
		{Name: "TLS", Fields: []plugin.Field{
			{Key: "tls_mode", Label: "TLS mode", Type: plugin.FieldSelect, Required: true, Default: "disable", Options: []plugin.Option{
				{Label: "Disable", Value: "disable"},
				{Label: "Require encryption", Value: "require"},
				{Label: "Verify CA", Value: "verify-ca"},
				{Label: "Verify full", Value: "verify-full"},
			}},
			{Key: "ca_certificate", Label: "CA certificate", Type: plugin.FieldTextarea, Secret: true, VisibleWhen: &verifyTLS, Help: "PEM CA bundle used for verify-ca and verify-full."},
			{Key: clientCertField, Label: "Client certificate", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialTLSClientCert}, Protocols: []string{protocolName},
			}, VisibleWhen: &tlsEnabled, Help: "Optional PEM containing the client certificate and private key."},
		}},
		{Name: "Safety", Fields: []plugin.Field{
			{Key: "read_only", Label: "Read-only mode", Type: plugin.FieldToggle, Default: true, Help: "Blocks writes and deletes from the key browser and terminal when enabled."},
			{Key: "require_write_confirmation", Label: "Confirm write commands", Type: plugin.FieldToggle, Default: true, Help: "Requires confirmation before write, delete, and administrative Redis commands execute from the command console."},
			{Key: "timeout", Label: "Command timeout", Type: plugin.FieldDuration, Default: defaultTimeout.String()},
			{Key: "pool_size", Label: "Pool size", Type: plugin.FieldNumber, Default: defaultPoolSize, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 50}}},
			{Key: "scan_count", Label: "Scan count", Type: plugin.FieldNumber, Default: defaultScanCount, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 10}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
			{Key: "value_limit", Label: "Collection read limit", Type: plugin.FieldNumber, Default: defaultValueLimit, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
			{Key: "key_pattern", Label: "Key pattern", Type: plugin.FieldText, Default: "*", Help: "Default SCAN pattern for the key browser."},
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
	database, ok := cfg.Int("database")
	if !ok {
		database = 0
	}
	if database < 0 {
		return options{}, fmt.Errorf("%w: database must be >= 0", plugin.ErrInvalidInput)
	}
	username := strings.TrimSpace(cfg.String("username"))
	if identity := strings.TrimSpace(cfg.String(service.CredentialIdentity)); identity != "" {
		username = identity
	}
	password := cfg.String("password")
	if secret := cfg.String(service.CredentialSecret); secret != "" {
		password = secret
	}
	scanCount := intValue(cfg.Config["scan_count"], defaultScanCount)
	if scanCount > plugin.MaxPageLimit {
		scanCount = plugin.MaxPageLimit
	}
	valueLimit := intValue(cfg.Config["value_limit"], defaultValueLimit)
	if valueLimit > plugin.MaxPageLimit {
		valueLimit = plugin.MaxPageLimit
	}
	pattern := strings.TrimSpace(cfg.String("key_pattern"))
	if pattern == "" {
		pattern = "*"
	}
	return options{
		Host:              host,
		Port:              port,
		Database:          database,
		Username:          username,
		Password:          password,
		TLSMode:           stringDefault(cfg.String("tls_mode"), "disable"),
		CACertificate:     cfg.String("ca_certificate"),
		ClientCertificate: cfg.String("_" + clientCertField + "_secret"),
		ReadOnly:          boolValue(cfg.Config["read_only"], true),
		RequireConfirm:    boolValue(cfg.Config["require_write_confirmation"], true),
		Timeout:           durationValue(cfg.Config["timeout"], defaultTimeout),
		PoolSize:          intValue(cfg.Config["pool_size"], defaultPoolSize),
		ScanCount:         scanCount,
		ValueLimit:        valueLimit,
		KeyPattern:        pattern,
	}, nil
}

func clientOptions(opts options, netTransport plugin.NetTransport) (*redisclient.Options, error) {
	tlsConfig, err := redisTLSConfig(opts)
	if err != nil {
		return nil, err
	}
	return &redisclient.Options{
		Addr:         net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
		Username:     opts.Username,
		Password:     opts.Password,
		DB:           opts.Database,
		PoolSize:     opts.PoolSize,
		DialTimeout:  opts.Timeout,
		ReadTimeout:  opts.Timeout,
		WriteTimeout: opts.Timeout,
		TLSConfig:    tlsConfig,
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return netTransport.DialContext(ctx, network, addr)
		},
	}, nil
}

func redisTLSConfig(opts options) (*tls.Config, error) {
	switch opts.TLSMode {
	case "", "disable":
		return nil, nil
	case "require", "verify-ca", "verify-full":
	default:
		return nil, fmt.Errorf("%w: unsupported TLS mode %q", plugin.ErrInvalidInput, opts.TLSMode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.TLSMode == "require" {
		cfg.InsecureSkipVerify = true //nolint:gosec // explicit Redis TLS mode matching SQL sslmode=require semantics.
	}
	if opts.TLSMode == "verify-full" {
		cfg.ServerName = opts.Host
	}
	if opts.CACertificate != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(opts.CACertificate)) {
			return nil, fmt.Errorf("%w: CA certificate is not valid PEM", plugin.ErrInvalidInput)
		}
		cfg.RootCAs = pool
	}
	if opts.ClientCertificate != "" {
		cert, err := tls.X509KeyPair([]byte(opts.ClientCertificate), []byte(opts.ClientCertificate))
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
