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
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charlesng35/shellcn/internal/plugin"
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

// RowMutation is the uniform request body for the editable data grid's
// insert/update/delete routes. Insert sends Values, Update sends Key+Values,
// Delete sends Key. Keys are the table's identifying columns; the renderer
// reads their values straight from the row it edited.
type RowMutation struct {
	Key    map[string]any `json:"key,omitempty"`
	Values map[string]any `json:"values,omitempty"`
}

// Placeholder formats a bind placeholder for the 1-based argument position so
// the row DML builder stays driver-neutral.
type Placeholder func(n int) string

// DollarPlaceholder formats $1, $2, … (PostgreSQL, CockroachDB).
func DollarPlaceholder(n int) string { return "$" + strconv.Itoa(n) }

// QuestionPlaceholder formats ? (MySQL/MariaDB, ClickHouse).
func QuestionPlaceholder(int) string { return "?" }

// ColonPlaceholder formats :1, :2, … (Oracle).
func ColonPlaceholder(n int) string { return ":" + strconv.Itoa(n) }

// AtPlaceholder formats @p1, @p2, … (SQL Server).
func AtPlaceholder(n int) string { return "@p" + strconv.Itoa(n) }

// IdentifierList parses a comma/whitespace separated list of column identifiers,
// validates each against the safe-identifier rule, and returns them quoted by
// the supplied quoter (used to build index column lists across dialects).
func IdentifierList(raw string, quote func(string) string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		id, err := SafeIdentifier(part)
		if err != nil {
			return nil, err
		}
		if quote != nil {
			id = quote(id)
		}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: at least one column is required", plugin.ErrInvalidInput)
	}
	return out, nil
}

// IdentifierListValue parses a column list supplied either as a multiselect array
// (the route-sourced options path) or a comma-separated string (free text), so a
// handler accepts both without caring how the form rendered the field.
func IdentifierListValue(v any, quote func(string) string) ([]string, error) {
	switch t := v.(type) {
	case string:
		return IdentifierList(t, quote)
	case []string:
		return IdentifierList(strings.Join(t, ","), quote)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, fmt.Sprint(item))
		}
		return IdentifierList(strings.Join(parts, ","), quote)
	default:
		return nil, fmt.Errorf("%w: columns must be a list of column names", plugin.ErrInvalidInput)
	}
}

// ValidateRowKey ensures a client-supplied key is exactly the table's primary
// key — same columns, no more, no fewer — so a row mutation can only ever target
// one identified row and a caller cannot turn an arbitrary column into a WHERE
// clause that sweeps many rows.
func ValidateRowKey(primaryKey []string, key map[string]any) error {
	if len(primaryKey) == 0 {
		return fmt.Errorf("%w: table has no primary key; rows cannot be edited", plugin.ErrForbidden)
	}
	if len(key) != len(primaryKey) {
		return fmt.Errorf("%w: row key must match the primary key exactly", plugin.ErrInvalidInput)
	}
	for _, col := range primaryKey {
		if _, ok := key[col]; !ok {
			return fmt.Errorf("%w: row key must match the primary key exactly", plugin.ErrInvalidInput)
		}
	}
	return nil
}

// AnyColumnRedacted reports whether any column matches a redaction pattern. Used
// to refuse exposing a primary key whose own value is sensitive (api_key, token…)
// — such tables stay read-only rather than leaking the raw key to the browser.
func AnyColumnRedacted(columns, patterns []string) bool {
	for _, c := range columns {
		if RedactColumn(c, patterns) {
			return true
		}
	}
	return false
}

// Dialect builds parameterized single-row DML for one driver's quoting and
// placeholder style. The table argument is supplied already quoted/qualified by
// the caller; QuoteIdent quotes a bare column identifier.
type Dialect struct {
	QuoteIdent  func(string) string
	Placeholder Placeholder
}

// Insert builds an INSERT for the given column values. Column order is stable
// (sorted) so the statement is deterministic and testable.
func (d Dialect) Insert(table string, values map[string]any) (string, []any, error) {
	if len(values) == 0 {
		return "", nil, fmt.Errorf("%w: no values to insert", plugin.ErrInvalidInput)
	}
	cols := sortedKeys(values)
	quoted := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		col, err := SafeIdentifier(c)
		if err != nil {
			return "", nil, err
		}
		quoted[i] = d.quote(col)
		placeholders[i] = d.Placeholder(i + 1)
		args[i] = normalizeArg(values[c])
	}
	stmt := "INSERT INTO " + table + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	return stmt, args, nil
}

// Update builds an UPDATE that sets values and matches the key columns. Both
// maps must be non-empty.
func (d Dialect) Update(table string, key, values map[string]any) (string, []any, error) {
	if len(values) == 0 {
		return "", nil, fmt.Errorf("%w: no values to update", plugin.ErrInvalidInput)
	}
	if len(key) == 0 {
		return "", nil, fmt.Errorf("%w: row key is required to update a row", plugin.ErrInvalidInput)
	}
	setCols := sortedKeys(values)
	args := make([]any, 0, len(setCols)+len(key))
	set := make([]string, len(setCols))
	n := 0
	for i, c := range setCols {
		col, err := SafeIdentifier(c)
		if err != nil {
			return "", nil, err
		}
		n++
		set[i] = d.quote(col) + " = " + d.Placeholder(n)
		args = append(args, normalizeArg(values[c]))
	}
	where, whereArgs, err := d.matchClause(key, &n)
	if err != nil {
		return "", nil, err
	}
	args = append(args, whereArgs...)
	stmt := "UPDATE " + table + " SET " + strings.Join(set, ", ") + " WHERE " + where
	return stmt, args, nil
}

// Delete builds a DELETE matching the key columns. The key must be non-empty so
// an editing mistake can never wipe a whole table.
func (d Dialect) Delete(table string, key map[string]any) (string, []any, error) {
	if len(key) == 0 {
		return "", nil, fmt.Errorf("%w: row key is required to delete a row", plugin.ErrInvalidInput)
	}
	n := 0
	where, args, err := d.matchClause(key, &n)
	if err != nil {
		return "", nil, err
	}
	return "DELETE FROM " + table + " WHERE " + where, args, nil
}

func (d Dialect) matchClause(key map[string]any, n *int) (string, []any, error) {
	cols := sortedKeys(key)
	parts := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		col, err := SafeIdentifier(c)
		if err != nil {
			return "", nil, err
		}
		*n++
		if key[c] == nil {
			parts[i] = d.quote(col) + " IS NULL"
			*n-- // no bound argument for NULL match
			args[i] = nil
			continue
		}
		parts[i] = d.quote(col) + " = " + d.Placeholder(*n)
		args[i] = normalizeArg(key[c])
	}
	// Drop the placeholder-less NULL matches from the argument list while
	// preserving order for the bound comparisons.
	bound := args[:0]
	for i, c := range cols {
		if key[c] != nil {
			bound = append(bound, args[i])
		}
	}
	return strings.Join(parts, " AND "), bound, nil
}

func (d Dialect) quote(col string) string {
	if d.QuoteIdent != nil {
		return d.QuoteIdent(col)
	}
	return QuoteIdent(col)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normalizeArg coerces integral JSON numbers (always float64 after decoding)
// back to int64 so integer/bigint columns and key comparisons bind cleanly.
func normalizeArg(v any) any {
	if f, ok := v.(float64); ok && !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f) && math.Abs(f) < 9.2e18 {
		return int64(f)
	}
	return v
}
