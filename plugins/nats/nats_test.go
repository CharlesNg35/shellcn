package nats

import (
	"context"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestNATSManifestValidates(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.MustRegister(New())

	proj, ok := reg.Projection(protocolName)
	if !ok {
		t.Fatal("projection missing")
	}
	if proj.Category.Key != plugin.CategoryMessaging {
		t.Fatalf("category: got %q want %q", proj.Category.Key, plugin.CategoryMessaging)
	}
	if proj.Layout != plugin.LayoutSidebarTree {
		t.Fatalf("layout: got %q", proj.Layout)
	}
	if len(proj.Resources) != 2 {
		t.Fatalf("resources: got %d", len(proj.Resources))
	}
}

func TestNATSConfigSchemaIsSpecific(t *testing.T) {
	m := New().Manifest()
	fields := fieldMap(m.Config)
	for _, key := range []string{"urls", "name", "auth", "username", "password", "token", "credential_id"} {
		if !fields[key] {
			t.Fatalf("missing field %q", key)
		}
	}
	for _, key := range []string{"management_url", "brokers"} {
		if fields[key] {
			t.Fatalf("nats should not expose %q", key)
		}
	}
}

func TestHealthCheckContextAddsDeadline(t *testing.T) {
	ctx, cancel := healthCheckContext(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("health check context should have a deadline")
	}
}

func TestHealthCheckContextKeepsExistingDeadline(t *testing.T) {
	base, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	want, _ := base.Deadline()
	ctx, done := healthCheckContext(base, 25*time.Millisecond)
	defer done()
	got, ok := ctx.Deadline()
	if !ok {
		t.Fatal("health check context should keep the caller deadline")
	}
	if !got.Equal(want) {
		t.Fatalf("deadline changed: got %v want %v", got, want)
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	out := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			out[field.Key] = true
		}
	}
	return out
}
