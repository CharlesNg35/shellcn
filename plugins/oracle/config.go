package oracle

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	go_ora "github.com/sijms/go-ora/v2"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

const (
	defaultPort       = 1521
	defaultRowLimit   = 500
	defaultTimeout    = 30 * time.Second
	defaultMaxConns   = 4
	protocolName      = "oracle"
	credentialIDField = "credential_id"
	authPassword      = "password"
	authCredential    = "credential"
)

type optionsData struct {
	Host           string
	Port           int
	Service        string
	SID            string
	Username       string
	Password       string
	TLSMode        string
	CACertificate  string
	DBAPrivilege   string
	ReadOnly       bool
	RequireConfirm bool
	QueryTimeout   time.Duration
	RowLimit       int
	MaxConns       int
	RedactPatterns []string
}

func configSchema() plugin.Schema {
	passwordAuth := plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authPassword}, {Field: credentialIDField, Op: plugin.OpEmpty}}}
	credentialAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: authCredential}, {Field: credentialIDField, Op: plugin.OpNotEmpty}}}
	verifyTLS := plugin.Condition{AnyOf: []plugin.Rule{
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-ca"},
		{Field: "tls_mode", Op: plugin.OpEq, Value: "verify-full"},
	}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "oracle.example.internal"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: defaultPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "service", Label: "Service name", Type: plugin.FieldText, Required: true, Default: "FREEPDB1"},
			{Key: "sid", Label: "SID", Type: plugin.FieldText, Help: "Optional legacy SID. When set it is used instead of service name."},
		}},
		{Name: "Authentication", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: authPassword, Options: []plugin.Option{
				{Label: "Password", Value: authPassword},
				{Label: "Stored credential", Value: authCredential},
			}},
			{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, Placeholder: "SYSTEM", VisibleWhen: &passwordAuth},
			{Key: credentialIDField, Label: "Stored password", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{plugin.CredentialDBPassword}, Protocols: []string{protocolName},
			}, VisibleWhen: &credentialAuth, Help: "Reusable Oracle password. The credential identity can also supply the username."},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &passwordAuth},
			{Key: "dba_privilege", Label: "DBA privilege", Type: plugin.FieldSelect, Default: "", Options: []plugin.Option{
				{Label: "None", Value: ""},
				{Label: "SYSDBA", Value: "SYSDBA"},
				{Label: "SYSOPER", Value: "SYSOPER"},
			}, Help: "Only use privileged modes for dedicated administrative connections."},
		}},
		{Name: "TLS", Fields: []plugin.Field{
			{Key: "tls_mode", Label: "TLS mode", Type: plugin.FieldSelect, Required: true, Default: "disable", Options: []plugin.Option{
				{Label: "Disable", Value: "disable"},
				{Label: "Require encryption", Value: "require"},
				{Label: "Verify CA", Value: "verify-ca"},
				{Label: "Verify full", Value: "verify-full"},
			}},
			{Key: "ca_certificate", Label: "CA certificate", Type: plugin.FieldTextarea, Secret: true, VisibleWhen: &verifyTLS, Help: "PEM CA bundle used for verify-ca and verify-full."},
		}},
		{Name: "Safety", Fields: []plugin.Field{
			{Key: "read_only", Label: "Read-only mode", Type: plugin.FieldToggle, Default: true, Help: "Blocks INSERT, UPDATE, DELETE, MERGE, PL/SQL blocks, DDL, TRUNCATE, GRANT, and other write statements."},
			{Key: "require_destructive_confirmation", Label: "Confirm destructive statements", Type: plugin.FieldToggle, Default: true, Help: "Requires explicit confirmation before write, DDL, PL/SQL, and privileged statements execute."},
			{Key: "query_timeout", Label: "Query timeout", Type: plugin.FieldDuration, Default: defaultTimeout.String()},
			{Key: "row_limit", Label: "Row limit", Type: plugin.FieldNumber, Default: defaultRowLimit, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: plugin.MaxPageLimit}}},
			{Key: "max_connections", Label: "Pool size", Type: plugin.FieldNumber, Default: defaultMaxConns, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 20}}},
			{Key: "redact_columns", Label: "Redacted columns", Type: plugin.FieldTextarea, Help: "Comma or newline separated regular expressions for result columns that must be masked."},
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
	serviceName := strings.TrimSpace(cfg.String("service"))
	sid := strings.TrimSpace(cfg.String("sid"))
	if serviceName == "" && sid == "" {
		serviceName = "FREEPDB1"
	}
	username := strings.TrimSpace(cfg.String("username"))
	if identity := strings.TrimSpace(cfg.String(service.CredentialIdentity)); identity != "" {
		username = identity
	}
	if username == "" {
		return optionsData{}, fmt.Errorf("%w: username is required", plugin.ErrInvalidInput)
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
	return optionsData{
		Host:           host,
		Port:           port,
		Service:        serviceName,
		SID:            sid,
		Username:       username,
		Password:       password,
		TLSMode:        stringDefault(cfg.String("tls_mode"), "disable"),
		CACertificate:  cfg.String("ca_certificate"),
		DBAPrivilege:   strings.ToUpper(strings.TrimSpace(cfg.String("dba_privilege"))),
		ReadOnly:       sqldb.BoolValue(cfg.Config["read_only"], true),
		RequireConfirm: sqldb.BoolValue(cfg.Config["require_destructive_confirmation"], true),
		QueryTimeout:   sqldb.DurationValue(cfg.Config["query_timeout"], defaultTimeout),
		RowLimit:       rowLimit,
		MaxConns:       maxConns,
		RedactPatterns: sqldb.ParsePatterns(cfg.String("redact_columns"), sqldb.DefaultRedactColumnPatterns()),
	}, nil
}

func openDB(opts optionsData, netTransport plugin.NetTransport) (*sql.DB, error) {
	urlOptions := map[string]string{
		"CONNECT TIMEOUT": strconv.Itoa(int(opts.QueryTimeout.Seconds())),
		"TIMEOUT":         strconv.Itoa(int(opts.QueryTimeout.Seconds())),
		"PREFETCH_ROWS":   strconv.Itoa(opts.RowLimit),
	}
	if opts.SID != "" {
		urlOptions["SID"] = opts.SID
	}
	if opts.DBAPrivilege != "" {
		switch opts.DBAPrivilege {
		case "SYSDBA", "SYSOPER":
			urlOptions["DBA PRIVILEGE"] = opts.DBAPrivilege
		default:
			return nil, fmt.Errorf("%w: unsupported DBA privilege %q", plugin.ErrInvalidInput, opts.DBAPrivilege)
		}
	}
	tlsConfig, ssl, verify, err := oracleTLSConfig(opts)
	if err != nil {
		return nil, err
	}
	if ssl {
		urlOptions["SSL"] = "true"
	}
	if verify {
		urlOptions["SSL VERIFY"] = "true"
	}
	connector, ok := go_ora.NewConnector(go_ora.BuildUrl(opts.Host, opts.Port, opts.Service, opts.Username, opts.Password, urlOptions)).(*go_ora.OracleConnector)
	if !ok {
		return nil, fmt.Errorf("%w: Oracle connector type is unavailable", plugin.ErrUnavailable)
	}
	connector.Dialer(netDialer{net: netTransport})
	if tlsConfig != nil {
		connector.WithTLSConfig(tlsConfig)
	}
	return sql.OpenDB(connector), nil
}

type netDialer struct {
	net plugin.NetTransport
}

func (d netDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.net.DialContext(ctx, network, addr)
}

func oracleTLSConfig(opts optionsData) (*tls.Config, bool, bool, error) {
	switch opts.TLSMode {
	case "", "disable":
		return nil, false, false, nil
	case "require", "verify-ca", "verify-full":
	default:
		return nil, false, false, fmt.Errorf("%w: unsupported TLS mode %q", plugin.ErrInvalidInput, opts.TLSMode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	verify := opts.TLSMode == "verify-ca" || opts.TLSMode == "verify-full"
	if opts.TLSMode == "require" {
		cfg.InsecureSkipVerify = true //nolint:gosec // explicit user-selected Oracle TCPS trust mode.
	}
	if opts.TLSMode == "verify-full" {
		cfg.ServerName = opts.Host
	}
	if opts.CACertificate != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(opts.CACertificate)) {
			return nil, false, false, fmt.Errorf("%w: CA certificate is not valid PEM", plugin.ErrInvalidInput)
		}
		cfg.RootCAs = pool
	}
	return cfg, true, verify, nil
}

func stringDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
