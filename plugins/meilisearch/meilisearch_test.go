package meilisearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/searchrest"
)

// wrappedSession mimics the core's borrowed session.Handle: a plugin.Session
// that exposes the live session via Session().
type wrappedSession struct{ inner plugin.Session }

func (w wrappedSession) Session() plugin.Session           { return w.inner }
func (w wrappedSession) HealthCheck(context.Context) error { return nil }
func (w wrappedSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}
func (w wrappedSession) Close() error { return nil }

func TestUnwrapResolvesThroughHandleWrapper(t *testing.T) {
	inner := &Session{}
	if got, err := unwrap(inner); err != nil || got != inner {
		t.Fatalf("bare session: got %v, err %v", got, err)
	}
	if got, err := unwrap(wrappedSession{inner: inner}); err != nil || got != inner {
		t.Fatalf("wrapped session must resolve to the inner session: got %v, err %v", got, err)
	}
}

// newMockSession points a real Session at an httptest server so route handlers
// run against representative API JSON without a live Meilisearch.
func newMockSession(t *testing.T, h http.HandlerFunc) (*Session, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	s := &Session{
		client: searchrest.New(searchrest.Options{Endpoint: srv.URL}),
		opts:   Options{PageLimit: 100},
	}
	return s, srv.Close
}

// Meilisearch returns /tasks "next" as a number (or null); decoding it must not
// fail, which previously 500'd the tasks list/tree.
func TestListTasksDecodesNumericNextCursor(t *testing.T) {
	s, closeSrv := newMockSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"uid":3,"type":"indexCreation","status":"succeeded"}],"total":1,"limit":20,"from":3,"next":2}`))
	})
	defer closeSrv()

	rc := plugin.NewRequestContext(context.Background(), models.User{}, s, nil, url.Values{}, nil)
	res, err := treeTasks(rc)
	if err != nil {
		t.Fatalf("treeTasks: %v", err)
	}
	page := res.(plugin.Page[plugin.TreeNode])
	if page.NextCursor != "2" {
		t.Fatalf("next cursor = %q, want %q", page.NextCursor, "2")
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %#v", page.Items)
	}
}

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()
	if err := plugin.Validate(m, p.Routes()); err != nil {
		t.Fatalf("manifest should validate: %v", err)
	}
	if m.Category != plugin.CategorySearch {
		t.Fatalf("category: got %q want %q", m.Category, plugin.CategorySearch)
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("meilisearch should be direct-only: %+v", m.SupportedTransports)
	}
	fields := fieldMap(m.Config)
	for _, key := range []string{"endpoint", "auth", "api_key", "credential_id", "tls_mode", "read_only", "page_limit"} {
		if !fields[key] {
			t.Fatalf("missing field %q", key)
		}
	}
	for _, key := range []string{"username", "password", "bearer_token"} {
		if fields[key] {
			t.Fatalf("unexpected field %q", key)
		}
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	fields := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			fields[field.Key] = true
		}
	}
	return fields
}
