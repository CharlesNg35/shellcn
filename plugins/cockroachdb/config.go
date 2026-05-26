package cockroachdb

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

const (
	defaultPort       = 26257
	defaultRowLimit   = 500
	defaultTimeout    = 30 * time.Second
	defaultMaxConns   = 4
	protocolName      = "cockroachdb"
	credentialIDField = "credential_id"
	clientCertField   = "client_cert_id"
	authPassword      = "password"
	authCredential    = "credential"
)

type options struct {
	Host              string
	Port              int
	Database          string
	Username          string
	Password          string
	TLSMode           string
	CACertificate     string
	ClientCertificate string
	ReadOnly          bool
	RequireConfirm    bool
	QueryTimeout      time.Duration
	RowLimit          int
	MaxConns          int
	ApplicationName   string
	RedactPatterns    []string
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
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "cockroach.example.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "database", Label: "Database", Type: plugin.FieldText, Required: true, Default: "defaultdb"},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: authPassword, Options: []plugin.Option{
				{Label: "Password", Value: authPassword},
				{Label: "Stored credential", Value: authCredential},
			}},
			{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, Placeholder: "root", VisibleWhen: &passwordAuth},
			{Key: credentialIDField, Label: "Stored password", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialDBPassword}, Protocols: []string{protocolName},
			}, VisibleWhen: &credentialAuth, Help: "Reusable database password. The credential identity can also supply the username."},
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
			{Key: "read_only", Label: "Read-only mode", Type: plugin.FieldToggle, Default: true, Help: "Blocks INSERT, UPSERT, UPDATE, DELETE, IMPORT, BACKUP, RESTORE, DDL, TRUNCATE, and other write statements."},
			{Key: "require_destructive_confirmation", Label: "Confirm destructive statements", Type: plugin.FieldToggle, Default: true, Help: "Requires explicit confirmation before write, DDL, job-control, backup/restore, grant, and revoke statements execute."},
			{Key: "query_timeout", Label: "Query timeout", Type: plugin.FieldDuration, Default: defaultTimeout.String()},
			{Key: "row_limit", Label: "Row limit", Type: plugin.FieldNumber, Default: defaultRowLimit, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
			{Key: "max_connections", Label: "Pool size", Type: plugin.FieldNumber, Default: defaultMaxConns, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 20}}},
			{Key: "redact_columns", Label: "Redacted columns", Type: plugin.FieldTextarea, Help: "Comma or newline separated regular expressions for result columns that must be masked."},
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
	database := strings.TrimSpace(cfg.String("database"))
	if database == "" {
		return options{}, fmt.Errorf("%w: database is required", plugin.ErrInvalidInput)
	}
	username := strings.TrimSpace(cfg.String("username"))
	if identity := strings.TrimSpace(cfg.String(service.CredentialIdentity)); identity != "" {
		username = identity
	}
	if username == "" {
		return options{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
	}
	password := cfg.String("password")
	if secret := cfg.String(service.CredentialSecret); secret != "" {
		password = secret
	}

	rowLimit, ok := cfg.Int("row_limit")
	if !ok || rowLimit <= 0 {
		rowLimit = defaultRowLimit
	}
	if rowLimit > plugin.MaxPageLimit {
		rowLimit = plugin.MaxPageLimit
	}
	maxConns, ok := cfg.Int("max_connections")
	if !ok || maxConns <= 0 {
		maxConns = defaultMaxConns
	}
	if maxConns > 20 {
		maxConns = 20
	}
	timeout := sqldb.DurationValue(cfg.Config["query_timeout"], defaultTimeout)
	return options{
		Host:              host,
		Port:              port,
		Database:          database,
		Username:          username,
		Password:          password,
		TLSMode:           stringDefault(cfg.String("tls_mode"), "disable"),
		CACertificate:     cfg.String("ca_certificate"),
		ClientCertificate: cfg.String("_" + clientCertField + "_secret"),
		ReadOnly:          sqldb.BoolValue(cfg.Config["read_only"], true),
		RequireConfirm:    sqldb.BoolValue(cfg.Config["require_destructive_confirmation"], true),
		QueryTimeout:      timeout,
		RowLimit:          rowLimit,
		MaxConns:          maxConns,
		ApplicationName:   "shellcn-cockroachdb",
		RedactPatterns:    sqldb.ParsePatterns(cfg.String("redact_columns"), sqldb.DefaultRedactColumnPatterns()),
	}, nil
}

func poolConfig(opts options, netTransport plugin.NetTransport) (*pgxpool.Config, error) {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(opts.Username, opts.Password),
		Host:   net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
		Path:   "/" + opts.Database,
	}
	q := u.Query()
	q.Set("application_name", opts.ApplicationName)
	q.Set("sslmode", opts.TLSMode)
	u.RawQuery = q.Encode()
	pc, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid CockroachDB connection config: %v", plugin.ErrInvalidInput, err)
	}
	pc.ConnConfig.DialFunc = netTransport.DialContext
	tlsConfig, err := sqldb.TLSConfig(sqldb.TLSOptions{
		Mode:              opts.TLSMode,
		Host:              opts.Host,
		CACertificate:     opts.CACertificate,
		ClientCertificate: opts.ClientCertificate,
	})
	if err != nil {
		return nil, err
	}
	pc.ConnConfig.TLSConfig = tlsConfig
	pc.MaxConns = int32(opts.MaxConns)
	pc.MinConns = 0
	pc.HealthCheckPeriod = 30 * time.Second
	pc.MaxConnIdleTime = 5 * time.Minute
	return pc, nil
}

func stringDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
