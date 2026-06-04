package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// User is the lean identity of the acting user handed to a route handler.
// Authorization is enforced by the core before the handler runs; a handler only
// needs identity for context and per-owner scoping (e.g. snippets).
type User struct {
	ID          string
	Username    string
	DisplayName string
	Roles       []string
}

// AuditResult is the outcome a handler reports for an audited operation that
// happens inside a long-lived route (e.g. one statement over a WebSocket).
type AuditResult string

const (
	AuditAllowed AuditResult = "allowed"
	AuditDenied  AuditResult = "denied"
	AuditError   AuditResult = "error"
)

// Snippet is a user-owned, per-protocol saved command exposed through the
// generic snippet routes.
type Snippet struct {
	ID        string
	OwnerID   string
	Protocol  string
	Name      string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Pagination defaults applied when a request omits or over-asks for a limit.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// SortKey is one ordering directive for a list route.
type SortKey struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// PageRequest carries cursor pagination, filtering, and sorting for list routes.
type PageRequest struct {
	Cursor string            `json:"cursor,omitempty"`
	Limit  int               `json:"limit,omitempty"`
	Filter map[string]string `json:"filter,omitempty"`
	Sort   []SortKey         `json:"sort,omitempty"`
}

// Search returns the table's free-text filter term (the grid's search box),
// trimmed. The "q" key is the contract — read it through here, never by hand.
func (p PageRequest) Search() string {
	return strings.TrimSpace(p.Filter["q"])
}

// Page is one slice of a paginated list.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor"`
	Total      *int   `json:"total,omitempty"`
}

// AuditHook records a plugin operation that happens inside a long-lived route,
// such as one query submitted over a WebSocket stream.
type AuditHook func(ctx context.Context, result AuditResult, params map[string]string, err error)

// SnippetStore is the small platform store surface exposed for generic snippet
// routes. It keeps plugins from depending on the concrete store package.
type SnippetStore interface {
	Create(ctx context.Context, s *Snippet) error
	Get(ctx context.Context, id string) (Snippet, error)
	ListByOwner(ctx context.Context, ownerID, protocol string) ([]Snippet, error)
	Update(ctx context.Context, s *Snippet) error
	Delete(ctx context.Context, id string) error
}

// UploadedFile is a multipart file part made available to route handlers. The
// file bytes are opened lazily so the audit/logging path never materializes them.
type UploadedFile struct {
	Field    string
	Filename string
	Size     int64
	Header   textproto.MIMEHeader

	header *multipart.FileHeader
}

// NewUploadedFile wraps a parsed multipart file header for RequestContext.
func NewUploadedFile(field string, header *multipart.FileHeader) UploadedFile {
	return UploadedFile{
		Field:    field,
		Filename: header.Filename,
		Size:     header.Size,
		Header:   header.Header,
		header:   header,
	}
}

// Open opens the uploaded file stream. Callers must close it.
func (f UploadedFile) Open() (multipart.File, error) {
	if f.header == nil {
		return nil, fmt.Errorf("%w: upload %q has no file handle", ErrInvalidInput, f.Field)
	}
	return f.header.Open()
}

var validate = validator.New(validator.WithRequiredStructEnabled())

// RequestContext gives a handler typed access to the request without ever
// touching http internals. Bind decodes + validates into a struct, so handlers
// never do panic-prone map[string]any assertions.
type RequestContext struct {
	Ctx      context.Context
	User     User
	Session  Session
	Snippets SnippetStore
	audit    AuditHook

	params map[string]string
	query  url.Values
	body   []byte
	form   url.Values
	files  map[string][]UploadedFile

	proxyPrefix string
}

// WithSnippets attaches the platform snippet store to a request context.
func (rc *RequestContext) WithSnippets(snippets SnippetStore) *RequestContext {
	rc.Snippets = snippets
	return rc
}

// WithProxyPrefix attaches the connection's public proxy mount (set by the
// core; the URL layout never lives in plugin code).
func (rc *RequestContext) WithProxyPrefix(prefix string) *RequestContext {
	rc.proxyPrefix = strings.TrimSuffix(prefix, "/")
	return rc
}

// ProxyPrefix returns the connection's public proxy mount as supplied by the
// core, without a trailing slash (empty when the core did not provide one).
func (rc *RequestContext) ProxyPrefix() string { return rc.proxyPrefix }

// ProxyURL builds the browser-facing "open in browser" URL for this connection:
// the core-supplied proxy mount plus path-escaped sub-segments and a trailing
// slash. Handlers return it from routes bound to Open: OpenURL actions instead
// of hardcoding the gateway's URL space.
func (rc *RequestContext) ProxyURL(sub ...string) string {
	var b strings.Builder
	b.WriteString(rc.proxyPrefix)
	for _, s := range sub {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	b.WriteByte('/')
	return b.String()
}

// WithAuditHook attaches the core audit writer for stream-internal operations.
func (rc *RequestContext) WithAuditHook(hook AuditHook) *RequestContext {
	rc.audit = hook
	return rc
}

// Audit records one operation inside this route. It is a no-op when no core
// audit hook is attached, which keeps plugin unit tests lightweight.
func (rc *RequestContext) Audit(result AuditResult, params map[string]string, err error) {
	if rc.audit != nil {
		rc.audit(rc.Ctx, result, params, err)
	}
}

// NewRequestContext builds a context for the server adapter and for tests.
func NewRequestContext(ctx context.Context, user User, sess Session, params map[string]string, query url.Values, body []byte) *RequestContext {
	return &RequestContext{
		Ctx:     ctx,
		User:    user,
		Session: sess,
		params:  params,
		query:   query,
		body:    body,
	}
}

// NewMultipartRequestContext builds a context for a multipart/form-data request.
func NewMultipartRequestContext(ctx context.Context, user User, sess Session, params map[string]string, query url.Values, form url.Values, files map[string][]UploadedFile) *RequestContext {
	return &RequestContext{
		Ctx:     ctx,
		User:    user,
		Session: sess,
		params:  params,
		query:   query,
		form:    form,
		files:   files,
	}
}

// Param returns a resolved renderer-supplied parameter. Route handlers use it
// for required path placeholders and optional scoped context values.
func (rc *RequestContext) Param(name string) string {
	if rc.params == nil {
		return ""
	}
	return rc.params[name]
}

// ParamList splits a param that carries several values joined by sep — e.g. a
// multiselect scope filter, read with sep = ScopeSeparator — dropping blanks. It
// returns nil when the param is absent, so a handler treats "no scope" as "all".
func (rc *RequestContext) ParamList(name, sep string) []string {
	raw := rc.Param(name)
	if raw == "" {
		return nil
	}
	out := make([]string, 0, strings.Count(raw, sep)+1)
	for _, p := range strings.Split(raw, sep) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Query returns the raw query values (list controls + p.* params).
func (rc *RequestContext) Query() url.Values {
	if rc.query == nil {
		return url.Values{}
	}
	return rc.query
}

// Params returns a copy of all resolved parameters.
func (rc *RequestContext) Params() map[string]string {
	out := make(map[string]string, len(rc.params))
	for k, v := range rc.params {
		out[k] = v
	}
	return out
}

// Body returns the raw request body.
func (rc *RequestContext) Body() []byte { return rc.body }

// Uploads returns multipart files for a form field. The returned slice is a copy
// so callers cannot mutate the request context.
func (rc *RequestContext) Uploads(field string) []UploadedFile {
	if rc.files == nil {
		return nil
	}
	files := rc.files[field]
	out := make([]UploadedFile, len(files))
	copy(out, files)
	return out
}

// UploadFields returns the names of multipart fields that carried files.
func (rc *RequestContext) UploadFields() []string {
	if rc.files == nil {
		return nil
	}
	fields := make([]string, 0, len(rc.files))
	for field := range rc.files {
		fields = append(fields, field)
	}
	return fields
}

// Bind decodes the request body into dst and runs struct-tag validation. JSON is
// the default; multipart form values are coerced into JSON-compatible values and
// uploaded bytes remain available through Uploads.
func (rc *RequestContext) Bind(dst any) error {
	body := rc.body
	if rc.form != nil {
		var err error
		body, err = json.Marshal(formValues(rc.form))
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, dst); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}
	if err := validate.Struct(dst); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return nil
}

// ValidateSchema applies the manifest-declared input schema at the core wrapper
// before the handler runs. Handlers still call Bind for typed decoding.
func (rc *RequestContext) ValidateSchema(schema *Schema) error {
	if schema == nil {
		return nil
	}
	values, err := rc.inputValues()
	if err != nil {
		return err
	}
	uploaded := make(map[string]bool, len(rc.files))
	for field := range rc.files {
		uploaded[field] = true
	}
	return schema.ValidateValues(values, uploaded)
}

// ValidateValues checks a decoded value map against the schema: per-field
// visibility, required-ness, type, options, and validators. File fields are
// satisfied by the caller-supplied uploaded set (the actual bytes live outside
// the value map). It is the shared validator behind the route wrapper
// (ValidateSchema) and the control-plane connection-config check.
func (s Schema) ValidateValues(values map[string]any, uploaded map[string]bool) error {
	return s.ValidateValuesWithContext(values, uploaded, nil)
}

func (s Schema) ValidateValuesWithContext(values map[string]any, uploaded map[string]bool, context map[string]any) error {
	conditionValues := mergedConditionValues(values, context)
	known := map[string]bool{}
	for _, group := range s.Groups {
		for _, field := range group.Fields {
			known[field.Key] = true
			if !visible(field.VisibleWhen, conditionValues) {
				continue
			}
			value, exists := values[field.Key]
			if field.Type == FieldFile {
				if field.Required && !uploaded[field.Key] && emptyValue(value, !exists) {
					return fmt.Errorf("%w: %s is required", ErrInvalidInput, field.Key)
				}
				continue
			}
			if field.Required && emptyValue(value, !exists) {
				return fmt.Errorf("%w: %s is required", ErrInvalidInput, field.Key)
			}
			if !exists || emptyValue(value, false) {
				continue
			}
			if err := validateFieldValue(field, value); err != nil {
				return err
			}
		}
	}
	for key := range values {
		if !known[key] {
			return fmt.Errorf("%w: unknown field %q", ErrInvalidInput, key)
		}
	}
	for key := range uploaded {
		if !known[key] {
			return fmt.Errorf("%w: unknown upload field %q", ErrInvalidInput, key)
		}
	}
	return nil
}

func (s Schema) VisibleValues(values map[string]any, context map[string]any) map[string]any {
	conditionValues := mergedConditionValues(values, context)
	out := map[string]any{}
	for _, group := range s.Groups {
		for _, field := range group.Fields {
			if !visible(field.VisibleWhen, conditionValues) {
				continue
			}
			if value, ok := values[field.Key]; ok {
				out[field.Key] = value
			}
		}
	}
	return out
}

func (s Schema) VisibleSecretKeys(values map[string]any, context map[string]any) []string {
	conditionValues := mergedConditionValues(values, context)
	var keys []string
	for _, group := range s.Groups {
		for _, field := range group.Fields {
			if field.Secret && visible(field.VisibleWhen, conditionValues) {
				keys = append(keys, field.Key)
			}
		}
	}
	return keys
}

func mergedConditionValues(values map[string]any, context map[string]any) map[string]any {
	if len(context) == 0 {
		return values
	}
	out := make(map[string]any, len(values)+len(context))
	for key, value := range values {
		out[key] = value
	}
	for key, value := range context {
		out[key] = value
	}
	return out
}

func (rc *RequestContext) inputValues() (map[string]any, error) {
	if rc.form != nil {
		return formValues(rc.form), nil
	}
	if len(rc.body) == 0 {
		return map[string]any{}, nil
	}
	var values map[string]any
	if err := json.Unmarshal(rc.body, &values); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if values == nil {
		values = map[string]any{}
	}
	return values, nil
}

func validateFieldValue(field Field, value any) error {
	switch field.Type {
	case FieldText, FieldEmail, FieldURL, FieldTel, FieldPassword, FieldTextarea, FieldDuration, FieldCredentialRef, FieldRadio:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: %s must be a string", ErrInvalidInput, field.Key)
		}
	case FieldNumber, FieldStepper, FieldSlider:
		if _, ok := numberValue(value); !ok {
			return fmt.Errorf("%w: %s must be a number", ErrInvalidInput, field.Key)
		}
	case FieldToggle:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: %s must be a boolean", ErrInvalidInput, field.Key)
		}
	case FieldMultiSelect:
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%w: %s must be a list", ErrInvalidInput, field.Key)
		}
	}
	if len(field.Options) > 0 && (field.Type == FieldSelect || field.Type == FieldMultiSelect || field.Type == FieldRadio) {
		if err := validateOptions(field, value); err != nil {
			return err
		}
	}
	for _, v := range field.Validators {
		if err := validateRule(field, v, value); err != nil {
			return err
		}
	}
	return nil
}

func validateOptions(field Field, value any) error {
	allowed := map[string]bool{}
	for _, option := range field.Options {
		allowed[fmt.Sprint(option.Value)] = true
	}
	if field.Type == FieldMultiSelect {
		items, _ := value.([]any)
		for _, item := range items {
			if !allowed[fmt.Sprint(item)] {
				return fmt.Errorf("%w: %s has an invalid option", ErrInvalidInput, field.Key)
			}
		}
		return nil
	}
	if !allowed[fmt.Sprint(value)] {
		return fmt.Errorf("%w: %s has an invalid option", ErrInvalidInput, field.Key)
	}
	return nil
}

func validateRule(field Field, rule Validator, value any) error {
	msg := rule.Message
	if msg == "" {
		msg = fmt.Sprintf("%s failed %s validation", field.Key, rule.Type)
	}
	fail := func() error { return fmt.Errorf("%w: %s", ErrInvalidInput, msg) }
	switch rule.Type {
	case ValidatorMin:
		minVal, ok := numberValue(rule.Value)
		if !ok {
			return nil
		}
		if n, ok := numberValue(value); ok && n < minVal {
			return fail()
		}
		if s, ok := value.(string); ok && float64(len(s)) < minVal {
			return fail()
		}
	case ValidatorMax:
		maxVal, ok := numberValue(rule.Value)
		if !ok {
			return nil
		}
		if n, ok := numberValue(value); ok && n > maxVal {
			return fail()
		}
		if s, ok := value.(string); ok && float64(len(s)) > maxVal {
			return fail()
		}
	case ValidatorRegex:
		pattern, ok := rule.Value.(string)
		if !ok {
			return nil
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("%w: invalid regex for %s", ErrInvalidInput, field.Key)
		}
		s, ok := value.(string)
		if !ok || !re.MatchString(s) {
			return fail()
		}
	case ValidatorOneOf:
		for _, item := range asList(rule.Value) {
			if fmt.Sprint(item) == fmt.Sprint(value) {
				return nil
			}
		}
		return fail()
	}
	return nil
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func visible(cond *Condition, values map[string]any) bool {
	if cond == nil {
		return true
	}
	for _, rule := range cond.AllOf {
		if !matchRule(rule, values[rule.Field]) {
			return false
		}
	}
	if len(cond.AnyOf) > 0 {
		for _, rule := range cond.AnyOf {
			if matchRule(rule, values[rule.Field]) {
				return true
			}
		}
		return false
	}
	return true
}

func matchRule(rule Rule, value any) bool {
	switch rule.Op {
	case OpEq:
		return fmt.Sprint(value) == fmt.Sprint(rule.Value)
	case OpNeq:
		return fmt.Sprint(value) != fmt.Sprint(rule.Value)
	case OpIn:
		for _, item := range asList(rule.Value) {
			if fmt.Sprint(value) == fmt.Sprint(item) {
				return true
			}
		}
		return false
	case OpNin:
		for _, item := range asList(rule.Value) {
			if fmt.Sprint(value) == fmt.Sprint(item) {
				return false
			}
		}
		return true
	case OpEmpty:
		return emptyValue(value, value == nil)
	case OpNotEmpty:
		return !emptyValue(value, value == nil)
	default:
		return false
	}
}

func asList(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case []int:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case []float64:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return []any{value}
	}
}

func emptyValue(value any, missing bool) bool {
	if missing || value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

func formValues(values url.Values) map[string]any {
	out := make(map[string]any, len(values))
	for key, vals := range values {
		if len(vals) == 1 {
			out[key] = parseFormScalar(vals[0])
			continue
		}
		items := make([]any, 0, len(vals))
		for _, value := range vals {
			items = append(items, parseFormScalar(value))
		}
		out[key] = items
	}
	return out
}

func parseFormScalar(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") ||
		trimmed == "true" || trimmed == "false" || trimmed == "null" ||
		(trimmed[0] >= '0' && trimmed[0] <= '9') || trimmed[0] == '-' {
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}
	return value
}

// Page parses cursor/limit/filter/sort from the query and clamps the limit.
func (rc *RequestContext) Page() (PageRequest, error) {
	q := rc.Query()
	p := PageRequest{Cursor: q.Get("cursor"), Limit: DefaultPageLimit}

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return PageRequest{}, fmt.Errorf("%w: limit must be an integer", ErrInvalidInput)
		}
		p.Limit = n
	}
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}

	for key, vals := range q {
		if len(vals) == 0 {
			continue
		}
		switch {
		case key == "filter":
			if p.Filter == nil {
				p.Filter = map[string]string{}
			}
			p.Filter["q"] = vals[0]
		case strings.HasPrefix(key, "filter."):
			if p.Filter == nil {
				p.Filter = map[string]string{}
			}
			p.Filter[strings.TrimPrefix(key, "filter.")] = vals[0]
		}
	}

	if raw := q.Get("sort"); raw != "" {
		desc := strings.HasPrefix(raw, "-")
		p.Sort = []SortKey{{Field: strings.TrimPrefix(raw, "-"), Desc: desc}}
	}
	return p, nil
}
