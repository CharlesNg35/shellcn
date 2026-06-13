package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	p := New()
	m := p.Manifest()
	plugintest.ValidatePlugin(t, p)
	if m.Agent != nil {
		t.Fatal("Redis must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialDBPassword) {
		t.Fatal("database password credential should support Redis")
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialTLSClientCert) {
		t.Fatal("TLS client certificate credential should support Redis")
	}
	if got := m.Config.Defaults()["read_only"]; got != true {
		t.Fatalf("read_only manifest default = %#v, want true", got)
	}
	if _, ok := m.Config.Defaults()["database"]; ok {
		t.Fatal("database should be selected from the workspace scope, not connection config")
	}
	if len(m.Scope) != 1 || m.Scope[0].Param != databaseScopeParam || m.Scope[0].DefaultValue != "0" {
		t.Fatalf("database scope not declared correctly: %+v", m.Scope)
	}
	var console *plugin.Panel
	for i := range m.Tabs {
		if m.Tabs[i].Key == "console" {
			console = &m.Tabs[i]
			break
		}
	}
	if console == nil || console.Type != plugin.PanelTerminal || console.Source == nil || console.Source.RouteID != "redis.terminal" {
		t.Fatalf("console should be a terminal panel backed by redis.terminal, got %+v", console)
	}
	if console.Type == plugin.PanelTerminalGrid {
		t.Fatal("redis console should stay a single terminal panel")
	}
	var info *plugin.Panel
	tabs := map[string]plugin.Panel{}
	for i := range m.Tabs {
		tabs[m.Tabs[i].Key] = m.Tabs[i]
		if m.Tabs[i].Key == "info" {
			info = &m.Tabs[i]
			break
		}
	}
	if info == nil || info.Type != plugin.PanelObjectDetail {
		t.Fatalf("info should render object details, got %+v", info)
	}
	if cfg, ok := info.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
		t.Fatalf("info config = %#v, want raw-toggle object detail", info.Config)
	}
	dash, ok := m.Tabs[0].Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("overview config = %T, want DashboardConfig", m.Tabs[0].Config)
	}
	if len(dash.Cells) == 0 || dash.Cells[0].Key != "server" || dash.Cells[0].Type != plugin.PanelObjectDetail {
		t.Fatalf("server dashboard cell = %+v, want object_detail", dash.Cells)
	}
	for _, key := range []string{"clients", "channels"} {
		tab, ok := tabs[key]
		if !ok {
			t.Fatalf("missing %s tab", key)
		}
		cfg, ok := tab.Config.(plugin.TableConfig)
		if tab.Type != plugin.PanelTable || !ok {
			t.Fatalf("%s tab should be a table, got %+v", key, tab)
		}
		if cfg.EmptyText == "" || cfg.RefreshIntervalMs == 0 || !cfg.Exportable || cfg.RowClick != plugin.RowClickDetail {
			t.Fatalf("%s tab table config is not review-ready: %#v", key, cfg)
		}
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
	_, err := executeCommandRequest(context.Background(), &Session{}, sqldb.QueryRequest{Query: "SELECT 1"})
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("SELECT should be handled by the Database scope, got %v", err)
	}
	_, err = executeCommandRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, sqldb.QueryRequest{Query: "DEL session:1"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeCommandRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, sqldb.QueryRequest{Query: "FLUSHDB"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestClosedSessionStopsBeforeSafetyChecks(t *testing.T) {
	s := &Session{opts: options{ReadOnly: true}}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err := executeCommandRequest(context.Background(), s, sqldb.QueryRequest{Query: "DEL session:1"})
	if !errors.Is(err, plugin.ErrUnavailable) {
		t.Fatalf("expected closed session error, got %v", err)
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

func TestSelectedDatabaseDefaultsAndValidates(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), plugin.User{}, nil, nil, nil, nil)
	db, err := selectedDatabase(rc, 0)
	if err != nil || db != 0 {
		t.Fatalf("default database = %d, err %v", db, err)
	}
	rc = plugin.NewRequestContext(context.Background(), plugin.User{}, nil, map[string]string{databaseScopeParam: "2"}, nil, nil)
	db, err = selectedDatabase(rc, 0)
	if err != nil || db != 2 {
		t.Fatalf("scoped database = %d, err %v", db, err)
	}
	rc = plugin.NewRequestContext(context.Background(), plugin.User{}, nil, map[string]string{databaseScopeParam: "-1"}, nil, nil)
	if _, err = selectedDatabase(rc, 0); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("negative database should be invalid, got %v", err)
	}
}

func TestAuthDefaultsToNone(t *testing.T) {
	m := New().Manifest()
	visible := m.Config.VisibleValues(m.Config.ValuesWithDefaults(map[string]any{}), nil)
	if visible["auth"] != authNone {
		t.Fatalf("default auth = %#v, want none", visible["auth"])
	}
	if _, ok := visible["username"]; ok {
		t.Fatal("username should be hidden when Redis auth is none")
	}
	if _, ok := visible["password"]; ok {
		t.Fatal("password should be hidden when Redis auth is none")
	}
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "127.0.0.1"}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "" || opts.Password != "" {
		t.Fatalf("default auth should not set credentials: %+v", opts)
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
