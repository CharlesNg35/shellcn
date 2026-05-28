package meilisearch

import (
	"context"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
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
