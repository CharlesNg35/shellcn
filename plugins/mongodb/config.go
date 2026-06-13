package mongodb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/charlesng35/shellcn/plugins/shared/dbcred"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	protocolName      = "mongodb"
	defaultPort       = 27017
	defaultTimeout    = 10 * time.Second
	defaultPoolSize   = 8
	defaultDocLimit   = 500
	credentialIDField = "credential_id"
	authCertField     = "auth_client_cert_id"
	clientCertField   = "client_cert_id"
	authPassword      = "password"
	authCredential    = "credential"
	authClientCert    = "client_certificate"
)

type optionsData struct {
	Host              string
	Port              int
	Database          string
	AuthSource        string
	AuthMechanism     string
	Username          string
	Password          string
	TLSMode           string
	CACertificate     string
	ClientCertificate string
	ReadOnly          bool
	RequireConfirm    bool
	Timeout           time.Duration
	PoolSize          int
	DocumentLimit     int
}

func configSchema() plugin.Schema {
	passwordAuth := plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authPassword}, {Field: credentialIDField, Op: plugin.OpEmpty}, {Field: authCertField, Op: plugin.OpEmpty}}}
	usernameAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authPassword}, {Field: "auth", Op: plugin.OpEq, Value: authClientCert}, {Field: authCertField, Op: plugin.OpNotEmpty}}}
	credentialAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authCredential}, {Field: credentialIDField, Op: plugin.OpNotEmpty}}}
	passwordMechanismAuth := plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpNeq, Value: authClientCert}, {Field: authCertField, Op: plugin.OpEmpty}}}
	optionalClientCertificate := plugin.Condition{AllOf: []plugin.Rule{{Field: "tls_mode", Op: plugin.OpNeq, Value: "disable"}, {Field: "auth", Op: plugin.OpNeq, Value: authClientCert}}}
	verifyTLS := plugin.Condition{AnyOf: []plugin.Rule{
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-ca"},
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-full"},
	}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "mongodb.example.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "database", Label: "Default database", Type: plugin.FieldText, Required: true, Default: "admin"},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: authPassword, Options: []plugin.Option{
				{Label: "Password", Value: authPassword},
				{Label: "Stored password", Value: authCredential},
				{Label: "Client certificate", Value: authClientCert},
			}},
			{Key: "username", Label: "Username", Type: plugin.FieldText, VisibleWhen: &usernameAuth},
			{Key: credentialIDField, Label: "Stored password", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: plugin.CredentialDBPassword, Protocols: []string{protocolName},
			}, VisibleWhen: &credentialAuth, Help: "Reusable MongoDB password. The credential identity can also supply the username."},
			{Key: authCertField, Label: "Client certificate", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: plugin.CredentialTLSClientCert, Protocols: []string{protocolName},
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authClientCert}}}, Help: "Reusable X.509 client certificate and private key."},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &passwordAuth},
			{Key: "auth_source", Label: "Auth source", Type: plugin.FieldText, Default: "admin", VisibleWhen: &passwordMechanismAuth},
			{Key: "auth_mechanism", Label: "Auth mechanism", Type: plugin.FieldSelect, VisibleWhen: &passwordMechanismAuth, Options: []plugin.Option{
				{Label: "Default", Value: ""},
				{Label: "SCRAM-SHA-256", Value: "SCRAM-SHA-256"},
				{Label: "SCRAM-SHA-1", Value: "SCRAM-SHA-1"},
				{Label: "PLAIN", Value: "PLAIN"},
			}},
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
				Kind: plugin.CredentialTLSClientCert, Protocols: []string{protocolName},
			}, VisibleWhen: &optionalClientCertificate, Help: "Optional PEM containing the client certificate and private key for mTLS when password authentication is used."},
		}},
		{Name: "Safety", Fields: []plugin.Field{
			{Key: "read_only", Label: "Read-only mode", Type: plugin.FieldToggle, Default: true, Help: "Blocks inserts, updates, deletes, collection drops, and write commands."},
			{Key: "require_write_confirmation", Label: "Confirm write commands", Type: plugin.FieldToggle, Default: true, Help: "Requires confirmation before write and administrative MongoDB commands execute from the console."},
			{Key: "timeout", Label: "Command timeout", Type: plugin.FieldDuration, Default: defaultTimeout.String()},
			{Key: "pool_size", Label: "Pool size", Type: plugin.FieldNumber, Default: defaultPoolSize, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 50}}},
			{Key: "document_limit", Label: "Document limit", Type: plugin.FieldNumber, Default: defaultDocLimit, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
		}},
	}}
}

func parseOptions(cfg plugin.ConnectConfig) (optionsData, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return optionsData{}, fmt.Errorf("%w: host is required", plugin.ErrInvalidInput)
	}
	port, ok := cfg.Int("port")
	if !ok || port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return optionsData{}, fmt.Errorf("%w: port must be between 1 and 65535", plugin.ErrInvalidInput)
	}
	database := strings.TrimSpace(cfg.String("database"))
	if database == "" {
		database = "admin"
	}
	tlsMode := stringDefault(cfg.String("tls_mode"), "disable")
	auth := dbcred.ApplyPasswordCredential(cfg, cfg.String("username"), cfg.String("password"))
	clientCertificate := dbcred.ResolvedSecret(cfg, clientCertField)
	authMechanism := strings.TrimSpace(cfg.String("auth_mechanism"))
	authSource := stringDefault(cfg.String("auth_source"), "admin")
	certAuthMode := cfg.String("auth") == authClientCert || dbcred.ResolvedSecret(cfg, authCertField) != ""
	if certAuthMode {
		certAuth := dbcred.ApplyClientCertificateCredential(cfg, authCertField, cfg.String("username"), tlsMode, "")
		auth.Username = certAuth.Username
		auth.Password = ""
		tlsMode = certAuth.TLSMode
		clientCertificate = certAuth.ClientCertificate
		authMechanism = "MONGODB-X509"
		authSource = "$external"
	}
	if certAuthMode && clientCertificate == "" {
		return optionsData{}, fmt.Errorf("%w: client certificate is required", plugin.ErrInvalidInput)
	}
	limit := intValue(cfg.Config["document_limit"], defaultDocLimit)
	if limit > plugin.MaxPageLimit {
		limit = plugin.MaxPageLimit
	}
	return optionsData{
		Host:              host,
		Port:              port,
		Database:          database,
		AuthSource:        authSource,
		AuthMechanism:     authMechanism,
		Username:          auth.Username,
		Password:          auth.Password,
		TLSMode:           tlsMode,
		CACertificate:     cfg.String("ca_certificate"),
		ClientCertificate: clientCertificate,
		ReadOnly:          boolValue(cfg.Config["read_only"], true),
		RequireConfirm:    boolValue(cfg.Config["require_write_confirmation"], true),
		Timeout:           durationValue(cfg.Config["timeout"], defaultTimeout),
		PoolSize:          intValue(cfg.Config["pool_size"], defaultPoolSize),
		DocumentLimit:     limit,
	}, nil
}

func clientOptions(opts optionsData, netTransport plugin.NetTransport) (*options.ClientOptions, error) {
	tlsConfig, err := mongoTLSConfig(opts)
	if err != nil {
		return nil, err
	}
	co := options.Client().
		ApplyURI("mongodb://" + net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))).
		SetAppName("ShellCN").
		SetConnectTimeout(opts.Timeout).
		SetServerSelectionTimeout(opts.Timeout).
		SetTimeout(opts.Timeout).
		SetMaxPoolSize(uint64(opts.PoolSize)).
		SetDialer(contextDialer{net: netTransport})
	if tlsConfig != nil {
		co.SetTLSConfig(tlsConfig)
	}
	if opts.Username != "" || opts.Password != "" || opts.AuthMechanism == "MONGODB-X509" {
		co.SetAuth(options.Credential{
			AuthMechanism: opts.AuthMechanism,
			AuthSource:    opts.AuthSource,
			Username:      opts.Username,
			Password:      opts.Password,
			PasswordSet:   opts.Password != "",
		})
	}
	return co, nil
}

type contextDialer struct {
	net plugin.NetTransport
}

func (d contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.net.DialContext(ctx, network, address)
}

func mongoTLSConfig(opts optionsData) (*tls.Config, error) {
	switch opts.TLSMode {
	case "", "disable":
		return nil, nil
	case "require", "verify-ca", "verify-full":
	default:
		return nil, fmt.Errorf("%w: unsupported TLS mode %q", plugin.ErrInvalidInput, opts.TLSMode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.TLSMode == "require" {
		cfg.InsecureSkipVerify = true //nolint:gosec // explicit TLS mode matching SQL/Redis require semantics.
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
