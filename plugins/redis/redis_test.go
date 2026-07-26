package redis

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"

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
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialKindDBPassword) {
		t.Fatal("database password credential should support Redis")
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialKindTLSClientCert) {
		t.Fatal("TLS client certificate credential should support Redis")
	}
	if got := m.Config.Defaults()["read_only"]; got != true {
		t.Fatalf("read_only manifest default = %#v, want true", got)
	}
	if _, ok := m.Config.Defaults()["database"]; ok {
		t.Fatal("database should be selected from the workspace scope, not connection config")
	}
	if len(m.Scope) != 1 || m.Scope[0].Param != databaseScopeParam || m.Scope[0].Control != plugin.ScopeSelect || m.Scope[0].DefaultValue != "0" {
		t.Fatalf("database scope not declared correctly: %+v", m.Scope)
	}
	if !m.Scope[0].DisableSearch {
		t.Fatalf("database scope should disable select search: %+v", m.Scope[0])
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
	var monitor *plugin.Panel
	for i := range m.Tabs {
		if m.Tabs[i].Key == "monitor" {
			monitor = &m.Tabs[i]
			break
		}
	}
	if monitor == nil || monitor.Type != plugin.PanelLogStream || monitor.Source == nil || monitor.Source.RouteID != "redis.monitor" || monitor.Source.Method != plugin.MethodWS {
		t.Fatalf("monitor should be a log stream backed by redis.monitor, got %+v", monitor)
	}
	if !hasStream(m.Streams, "redis.monitor", plugin.StreamLogs, "redis.monitor") {
		t.Fatalf("missing redis.monitor log stream: %+v", m.Streams)
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
	} else if len(cfg.Sections) < 3 {
		t.Fatalf("info should expose structured overview sections, got %#v", cfg.Sections)
	} else if hasSection(cfg, "Memory") {
		t.Fatalf("info should not render Redis memory as a duplicate object detail card: %#v", cfg.Sections)
	}
	dash, ok := m.Tabs[0].Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("overview config = %T, want DashboardConfig", m.Tabs[0].Config)
	}
	if len(dash.Cells) == 0 || dash.Cells[0].Key != "server" || dash.Cells[0].Type != plugin.PanelObjectDetail {
		t.Fatalf("server dashboard cell = %+v, want object_detail", dash.Cells)
	}
	if len(dash.Cells) != 1 {
		t.Fatalf("overview should stay compact and avoid duplicating Clients/Channels tabs: %+v", dash.Cells)
	}
	if cfg, ok := dash.Cells[0].Config.(plugin.ObjectDetailConfig); !ok {
		t.Fatalf("overview server config = %T, want object detail", dash.Cells[0].Config)
	} else if cfg.RawToggle || hasSection(cfg, "Stats") {
		t.Fatalf("overview server summary should not duplicate the Info tab: %#v", cfg)
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

func hasStream(streams []plugin.Stream, id string, kind plugin.StreamKind, routeID string) bool {
	for _, stream := range streams {
		if stream.ID == id && stream.Kind == kind && stream.RouteID == routeID {
			return true
		}
	}
	return false
}

func TestRedisClientTableShowsOperationalColumns(t *testing.T) {
	cols := map[string]plugin.Column{}
	for _, col := range clientColumns() {
		cols[col.Key] = col
	}
	for _, key := range []string{"flags", "sub", "psub", "omem"} {
		if _, ok := cols[key]; !ok {
			t.Fatalf("client table missing %s column", key)
		}
	}
	if cols["omem"].Type != plugin.ColumnBytes {
		t.Fatalf("output memory should render as bytes: %#v", cols["omem"])
	}
}

func TestOverviewInfoKeysCoverOperationalSummary(t *testing.T) {
	keys := map[string]bool{}
	for _, key := range overviewInfoKeys() {
		keys[key] = true
	}
	for _, key := range []string{
		"role",
		"connected_clients",
		"blocked_clients",
		"used_memory",
		"used_memory_peak",
		"used_memory_human",
		"used_memory_peak_human",
		"instantaneous_ops_per_sec",
		"keyspace_hits",
		"keyspace_misses",
	} {
		if !keys[key] {
			t.Fatalf("overview payload should include %s", key)
		}
	}
}

func hasSection(cfg plugin.ObjectDetailConfig, title string) bool {
	for _, section := range cfg.Sections {
		if section.Title == title {
			return true
		}
	}
	return false
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

func TestMonitorUsesDedicatedNoReadTimeoutClient(t *testing.T) {
	base := redisclient.NewClient(&redisclient.Options{
		Addr:         "127.0.0.1:6379",
		DB:           2,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     8,
	})
	defer func() { _ = base.Close() }()
	s := &Session{client: base}

	mc := newMonitorConn(s, 3)
	defer mc.Close()
	if mc.conn == nil {
		t.Fatal("monitor must use a dedicated sticky connection")
	}
	opts := mc.client.Options()
	if opts.DB != 3 {
		t.Fatalf("monitor DB = %d, want 3", opts.DB)
	}
	if opts.PoolSize != 1 {
		t.Fatalf("monitor pool size = %d, want 1", opts.PoolSize)
	}
	if opts.ReadTimeout != 0 {
		t.Fatalf("monitor read timeout = %s, want no timeout", opts.ReadTimeout)
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

type fakeKeyspace struct {
	keys  []string
	batch int
	calls int
	err   error
}

func (f *fakeKeyspace) scan(_ context.Context, cursor uint64, match string, _ int64) ([]string, uint64, error) {
	f.calls++
	if f.err != nil {
		return nil, 0, f.err
	}
	start := int(cursor)
	end := min(start+f.batch, len(f.keys))
	found := make([]string, 0, end-start)
	for _, key := range f.keys[start:end] {
		if matchesGlob(match, key) {
			found = append(found, key)
		}
	}
	if end >= len(f.keys) {
		return found, 0, nil
	}
	return found, uint64(end), nil
}

// matchesGlob covers the subset of Redis MATCH globs the tests rely on.
func matchesGlob(pattern, key string) bool {
	switch {
	case pattern == "" || pattern == "*":
		return true
	case strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 1:
		return strings.Contains(key, strings.Trim(pattern, "*"))
	case strings.HasSuffix(pattern, "*"):
		return strings.HasPrefix(key, strings.TrimSuffix(pattern, "*"))
	default:
		return pattern == key
	}
}

func generatedKeys(n int, prefix string) []string {
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		keys = append(keys, prefix+strconv.Itoa(i))
	}
	return keys
}

func TestScanKeyPageStopsAtBatchCapOnSelectivePattern(t *testing.T) {
	// A pattern that matches nothing must not walk the whole keyspace in one page.
	ks := &fakeKeyspace{keys: generatedKeys(100_000, "session:"), batch: 200}
	keys, next, err := scanKeyPage(context.Background(), ks.scan, 0, "*nomatch*", 200)
	if err != nil {
		t.Fatalf("scan page: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected no matches, got %d", len(keys))
	}
	if ks.calls != maxScanBatchesPerPage {
		t.Fatalf("scan round trips = %d, want the %d batch cap", ks.calls, maxScanBatchesPerPage)
	}
	if next == 0 {
		t.Fatal("a short page must return a resume cursor instead of finishing the keyspace")
	}
	if want := uint64(maxScanBatchesPerPage * ks.batch); next != want {
		t.Fatalf("resume cursor = %d, want %d", next, want)
	}
}

func TestScanKeyPageResumesFromCursorAndTerminates(t *testing.T) {
	total := 100_000
	ks := &fakeKeyspace{keys: generatedKeys(total, "session:"), batch: 200}
	pages, seen := 0, 0
	cursor := uint64(0)
	for {
		keys, next, err := scanKeyPage(context.Background(), ks.scan, cursor, "*nomatch*", 200)
		if err != nil {
			t.Fatalf("scan page: %v", err)
		}
		seen += len(keys)
		pages++
		if next == 0 {
			break
		}
		if next <= cursor {
			t.Fatalf("cursor did not advance: %d -> %d", cursor, next)
		}
		cursor = next
		if pages > total {
			t.Fatal("paging did not terminate")
		}
	}
	if seen != 0 {
		t.Fatalf("selective pattern matched %d keys, want 0", seen)
	}
	if want := total / (ks.batch * maxScanBatchesPerPage); pages != want {
		t.Fatalf("pages = %d, want %d", pages, want)
	}
}

func TestScanKeyPageStopsAtLimit(t *testing.T) {
	ks := &fakeKeyspace{keys: generatedKeys(10_000, "user:"), batch: 50}
	keys, next, err := scanKeyPage(context.Background(), ks.scan, 0, "*", 200)
	if err != nil {
		t.Fatalf("scan page: %v", err)
	}
	if len(keys) < 200 {
		t.Fatalf("page returned %d keys, want at least the 200 limit", len(keys))
	}
	if len(keys) > 200+ks.batch {
		t.Fatalf("page returned %d keys, overshoot must stay within one batch of the limit", len(keys))
	}
	if ks.calls > maxScanBatchesPerPage {
		t.Fatalf("scan round trips = %d, want at most %d", ks.calls, maxScanBatchesPerPage)
	}
	if next == 0 {
		t.Fatal("a truncated page must return a resume cursor")
	}
}

func TestScanKeyPageFinishesSmallKeyspaceInOneCall(t *testing.T) {
	ks := &fakeKeyspace{keys: generatedKeys(12, "user:"), batch: 200}
	keys, next, err := scanKeyPage(context.Background(), ks.scan, 0, "*", 200)
	if err != nil {
		t.Fatalf("scan page: %v", err)
	}
	if len(keys) != 12 || next != 0 || ks.calls != 1 {
		t.Fatalf("small keyspace: keys=%d next=%d calls=%d", len(keys), next, ks.calls)
	}
}

func TestScanKeyPageDedupesRepeatedKeys(t *testing.T) {
	ks := &fakeKeyspace{keys: []string{"a", "b", "a", "b", "c"}, batch: 2}
	keys, next, err := scanKeyPage(context.Background(), ks.scan, 0, "*", 200)
	if err != nil {
		t.Fatalf("scan page: %v", err)
	}
	if next != 0 {
		t.Fatalf("next cursor = %d, want 0", next)
	}
	if len(keys) != 3 {
		t.Fatalf("keys = %#v, want deduped a/b/c", keys)
	}
}

func TestScanKeyPageWrapsScanErrors(t *testing.T) {
	ks := &fakeKeyspace{keys: generatedKeys(10, "user:"), batch: 5, err: errors.New("connection reset")}
	if _, _, err := scanKeyPage(context.Background(), ks.scan, 0, "*", 200); !errors.Is(err, plugin.ErrUnavailable) {
		t.Fatalf("scan error = %v, want plugin.ErrUnavailable", err)
	}
}

func TestKeyPageLimitStaysBounded(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested int
		scanCount int
		want      int
	}{
		{"defaults when unset", 0, 0, defaultScanCount},
		{"clamps to scan count", 500, 200, 200},
		{"honors smaller request", 50, 200, 50},
		{"negative falls back", -10, 200, 200},
		{"hard ceiling beats scan count", plugin.MaxPageLimit * 4, plugin.MaxPageLimit * 4, maxKeysPerPage},
	} {
		if got := keyPageLimit(tc.requested, tc.scanCount); got != tc.want {
			t.Fatalf("%s: keyPageLimit(%d, %d) = %d, want %d", tc.name, tc.requested, tc.scanCount, got, tc.want)
		}
	}
}

func TestKeyScanPatternFiltersServerSide(t *testing.T) {
	for _, tc := range []struct{ search, configured, want string }{
		{"", "", "*"},
		{"", "app:*", "app:*"},
		{"user", "app:*", "*user*"},
		{"user:*", "app:*", "user:*"},
		{"  session  ", "", "*session*"},
		{"cache:?", "", "cache:?"},
	} {
		if got := keyScanPattern(tc.search, tc.configured); got != tc.want {
			t.Fatalf("keyScanPattern(%q, %q) = %q, want %q", tc.search, tc.configured, got, tc.want)
		}
	}
}
