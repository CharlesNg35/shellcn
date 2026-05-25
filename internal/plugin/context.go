package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/charlesng/shellcn/internal/models"
)

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

// Page is one slice of a paginated list.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor"`
	Total      *int   `json:"total,omitempty"`
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
	Ctx     context.Context
	User    models.User
	Session Session

	params map[string]string
	query  url.Values
	body   []byte
	form   url.Values
	files  map[string][]UploadedFile
}

// NewRequestContext builds a context for the server adapter and for tests.
func NewRequestContext(ctx context.Context, user models.User, sess Session, params map[string]string, query url.Values, body []byte) *RequestContext {
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
func NewMultipartRequestContext(ctx context.Context, user models.User, sess Session, params map[string]string, query url.Values, form url.Values, files map[string][]UploadedFile) *RequestContext {
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

// Param returns a resolved path parameter (filled from the route template).
func (rc *RequestContext) Param(name string) string {
	if rc.params == nil {
		return ""
	}
	return rc.params[name]
}

// Query returns the raw query values (list controls + p.* params).
func (rc *RequestContext) Query() url.Values {
	if rc.query == nil {
		return url.Values{}
	}
	return rc.query
}

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
