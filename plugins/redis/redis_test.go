package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register Redis plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("Redis must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support Redis")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support Redis")
	}
	if err := plugin.Validate(m, New().Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	var console *plugin.Tab
	for i := range m.Tabs {
		if m.Tabs[i].Key == "console" {
			console = &m.Tabs[i]
			break
		}
	}
	if console == nil || console.Panel != plugin.PanelTerminal || console.Source == nil || console.Source.RouteID != "redis.terminal" {
		t.Fatalf("console should be a terminal panel backed by redis.terminal, got %+v", console)
	}
}

func TestParseCommand(t *testing.T) {
	got, err := parseCommand(`SET "user:1" 'Ada Lovelace'`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	want := []string{"SET", "user:1", "Ada Lovelace"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	if _, err := parseCommand(`GET "unterminated`); err == nil {
		t.Fatal("unterminated quote accepted")
	}
}

func TestCommandSafetyStopsBeforeRedis(t *testing.T) {
	_, err := executeCommandRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, sqldb.QueryRequest{Query: "DEL session:1"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeCommandRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, sqldb.QueryRequest{Query: "FLUSHDB"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestReadOnlyModeDefaultsOn(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "127.0.0.1"}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if !opts.ReadOnly {
		t.Fatal("read-only mode should be enabled by default")
	}
	opts, err = parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "127.0.0.1", "read_only": false}})
	if err != nil {
		t.Fatalf("parse options with read_only: %v", err)
	}
	if opts.ReadOnly {
		t.Fatal("read-only mode should be disabled when configured")
	}
}

func TestValueParsers(t *testing.T) {
	m, err := stringMapValue(`{"name":"ada","role":"admin"}`)
	if err != nil || m["name"] != "ada" || m["role"] != "admin" {
		t.Fatalf("unexpected map parse: %#v %v", m, err)
	}
	list, err := stringSliceValue("[\"a\",\"b\"]")
	if err != nil || len(list) != 2 || list[1] != "b" {
		t.Fatalf("unexpected list parse: %#v %v", list, err)
	}
	zs, err := zsetValue(`[{"member":"a","score":1.5},{"member":"b","score":2}]`)
	if err != nil || len(zs) != 2 || zs[0].Member != "a" || zs[0].Score != 1.5 {
		t.Fatalf("unexpected zset parse: %#v %v", zs, err)
	}
}
