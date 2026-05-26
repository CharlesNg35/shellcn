// Package sqldb contains SQL plugin helpers that are independent of a specific
// database driver.
package sqldb

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charlesng/shellcn/internal/plugin"
)

const IdentifierPattern = `^[A-Za-z_][A-Za-z0-9_]{0,62}$`

var identifierRE = regexp.MustCompile(IdentifierPattern)

const RedactedValue = "***"

type QueryRequest struct {
	Query     string `json:"query"`
	Confirm   bool   `json:"confirm,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type QueryResult struct {
	Columns    []string          `json:"columns"`
	Rows       [][]any           `json:"rows"`
	RowCount   int64             `json:"rowCount,omitempty"`
	ElapsedMS  int64             `json:"elapsedMs"`
	Statement  string            `json:"statement,omitempty"`
	CommandTag string            `json:"commandTag,omitempty"`
	Statements []StatementResult `json:"statements,omitempty"`
}

type StatementResult struct {
	Columns    []string `json:"columns"`
	Rows       [][]any  `json:"rows"`
	RowCount   int64    `json:"rowCount,omitempty"`
	ElapsedMS  int64    `json:"elapsedMs"`
	Statement  string   `json:"statement"`
	CommandTag string   `json:"commandTag,omitempty"`
}

type ColumnSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
	Primary  bool   `json:"primary"`
	Unique   bool   `json:"unique"`
}

type TLSOptions struct {
	Mode              string
	Host              string
	CACertificate     string
	ClientCertificate string
}

type CompletionItem struct {
	Label  string `json:"label"`
	Type   string `json:"type,omitempty"`
	Detail string `json:"detail,omitempty"`
	Apply  string `json:"apply,omitempty"`
}

type QueryAudit struct {
	Query          string
	Statements     []string
	Confirmed      bool
	ReadOnlyMode   bool
	RequiresReview bool
	RowCount       int64
	ElapsedMS      int64
	CommandTag     string
}

func SafeIdentifier(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !identifierRE.MatchString(raw) {
		return "", fmt.Errorf("%w: invalid identifier %q", plugin.ErrInvalidInput, raw)
	}
	return raw, nil
}

func OptionalIdentifier(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	return SafeIdentifier(raw)
}

func QuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func Qualified(schema, name string) string {
	return QuoteIdent(schema) + "." + QuoteIdent(name)
}

func ParseDDLColumns(value any) ([]string, error) {
	raw, err := NormalizeJSONValue(value)
	if err != nil {
		return nil, err
	}
	var specs []ColumnSpec
	if err := json.Unmarshal(raw, &specs); err != nil || len(specs) == 0 {
		return nil, fmt.Errorf("%w: columns must be a non-empty JSON array", plugin.ErrInvalidInput)
	}
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		col, err := DDLColumn(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, nil
}

func NormalizeJSONValue(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return nil, fmt.Errorf("%w: JSON value is empty", plugin.ErrInvalidInput)
		}
		return []byte(raw), nil
	case json.RawMessage:
		return v, nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid JSON value", plugin.ErrInvalidInput)
		}
		return raw, nil
	}
}

func DDLColumn(spec ColumnSpec) (string, error) {
	name, err := SafeIdentifier(spec.Name)
	if err != nil {
		return "", err
	}
	dataType := strings.TrimSpace(spec.Type)
	if !SafeType(dataType) {
		return "", fmt.Errorf("%w: unsafe column type", plugin.ErrInvalidInput)
	}
	parts := []string{QuoteIdent(name), dataType}
	if !spec.Nullable || spec.Primary {
		parts = append(parts, "NOT NULL")
	}
	if strings.TrimSpace(spec.Default) != "" {
		if !SafeDefault(spec.Default) {
			return "", fmt.Errorf("%w: unsafe default expression", plugin.ErrInvalidInput)
		}
		parts = append(parts, "DEFAULT "+strings.TrimSpace(spec.Default))
	}
	if spec.Primary {
		parts = append(parts, "PRIMARY KEY")
	}
	if spec.Unique {
		parts = append(parts, "UNIQUE")
	}
	return strings.Join(parts, " "), nil
}

func SafeType(s string) bool {
	if s == "" || strings.Contains(s, ";") || strings.Contains(s, "--") || strings.Contains(s, "/*") {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && !strings.ContainsRune("_(),.[]", r) {
			return false
		}
	}
	return true
}

func SafeDefault(s string) bool {
	return !strings.Contains(s, ";") && !strings.Contains(s, "--") && !strings.Contains(s, "/*") && !strings.Contains(s, "*/")
}

func SplitStatements(sqlText string) []string {
	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range sqlText {
		if quote != 0 {
			b.WriteRune(r)
			if r == quote && !escaped {
				quote = 0
			}
			escaped = r == '\\' && !escaped
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			b.WriteRune(r)
		case ';':
			if st := strings.TrimSpace(b.String()); st != "" {
				out = append(out, st)
			}
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if st := strings.TrimSpace(b.String()); st != "" {
		out = append(out, st)
	}
	return out
}

func FirstKeyword(statement string) string {
	statement = strings.TrimSpace(statement)
	for strings.HasPrefix(statement, "--") {
		if i := strings.IndexByte(statement, '\n'); i >= 0 {
			statement = strings.TrimSpace(statement[i+1:])
		} else {
			return ""
		}
	}
	fields := strings.Fields(statement)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func IsReadOnlyStatement(statement string) bool {
	switch FirstKeyword(statement) {
	case "SELECT", "SHOW", "EXPLAIN", "WITH", "VALUES", "DESCRIBE", "DESC":
		return true
	default:
		return false
	}
}

func IsDestructiveStatement(statement string) bool {
	switch FirstKeyword(statement) {
	case "DELETE", "DROP", "TRUNCATE", "ALTER", "UPDATE", "INSERT", "CREATE", "REINDEX", "VACUUM", "GRANT", "REVOKE", "OPTIMIZE", "ANALYZE", "LOCK", "UNLOCK", "CALL":
		return true
	default:
		return false
	}
}

func BoolValue(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func DurationValue(v any, def time.Duration) time.Duration {
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

func TLSConfig(opts TLSOptions) (*tls.Config, error) {
	switch opts.Mode {
	case "", "disable":
		return nil, nil
	case "require", "verify-ca", "verify-full":
	default:
		return nil, fmt.Errorf("%w: unsupported TLS mode %q", plugin.ErrInvalidInput, opts.Mode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.Mode == "require" {
		cfg.InsecureSkipVerify = true //nolint:gosec // matches common SQL sslmode=require semantics.
	}
	if opts.Mode == "verify-full" {
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

func DefaultRedactColumnPatterns() []string {
	return []string{
		`(?i)password`,
		`(?i)passwd`,
		`(?i)secret`,
		`(?i)token`,
		`(?i)api[_-]?key`,
		`(?i)private[_-]?key`,
		`(?i)credential`,
		`(?i)session`,
		`(?i)cookie`,
	}
}

func ParsePatterns(raw string, fallback []string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), fallback...)
	}
	return out
}

func RedactRows(columns []string, rows [][]any, patterns []string) [][]any {
	if len(rows) == 0 || len(columns) == 0 || len(patterns) == 0 {
		return rows
	}
	redact := make(map[int]bool)
	for i, column := range columns {
		if RedactColumn(column, patterns) {
			redact[i] = true
		}
	}
	if len(redact) == 0 {
		return rows
	}
	out := make([][]any, len(rows))
	for i, row := range rows {
		next := make([]any, len(row))
		copy(next, row)
		for idx := range redact {
			if idx < len(next) && next[idx] != nil {
				next[idx] = RedactedValue
			}
		}
		out[i] = next
	}
	return out
}

func RedactColumn(column string, patterns []string) bool {
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err == nil && re.MatchString(column) {
			return true
		}
	}
	return false
}

func AuditParams(in QueryAudit) map[string]string {
	params := map[string]string{
		"query_sha256":    QueryHash(in.Query),
		"statement_count": strconv.Itoa(len(in.Statements)),
		"confirmed":       strconv.FormatBool(in.Confirmed),
		"read_only_mode":  strconv.FormatBool(in.ReadOnlyMode),
	}
	if len(in.Statements) > 0 {
		params["first_statement"] = FirstKeyword(in.Statements[0])
	}
	if in.RequiresReview {
		params["requires_review"] = "true"
	}
	if in.RowCount > 0 {
		params["row_count"] = strconv.FormatInt(in.RowCount, 10)
	}
	if in.ElapsedMS > 0 {
		params["elapsed_ms"] = strconv.FormatInt(in.ElapsedMS, 10)
	}
	if in.CommandTag != "" {
		params["command_tag"] = in.CommandTag
	}
	return params
}

func QueryHash(query string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:])
}
