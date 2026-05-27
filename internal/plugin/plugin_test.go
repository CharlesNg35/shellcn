package plugin_test

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

type stubPlugin struct {
	manifest plugin.Manifest
	routes   []plugin.Route
}

func (s *stubPlugin) Manifest() plugin.Manifest { return s.manifest }
func (s *stubPlugin) Routes() []plugin.Route    { return s.routes }
func (s *stubPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

func TestRegistryRegisterGetAll(t *testing.T) {
	m, routes := sampleManifest()
	reg := plugin.NewRegistry()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); !errors.Is(err, plugin.ErrAlreadyExists) {
		t.Fatalf("duplicate register: want ErrAlreadyExists, got %v", err)
	}
	if _, ok := reg.Get("sample"); !ok {
		t.Error("Get(sample) not found")
	}
	if all := reg.All(); len(all) != 1 {
		t.Errorf("All: want 1, got %d", len(all))
	}
	if rt, ok := reg.Route("sample", "sample.start"); !ok || rt.Risk != plugin.RiskWrite {
		t.Errorf("Route lookup failed: ok=%v risk=%v", ok, rt.Risk)
	}
	if s := reg.Summaries(); len(s) != 1 || s[0].Name != "sample" {
		t.Errorf("Summaries unexpected: %+v", s)
	} else if s[0].Category.Key != plugin.CategoryShell {
		t.Errorf("Summary category = %+v, want %q", s[0].Category, plugin.CategoryShell)
	}
}

func TestRegistrySummariesSortByCategory(t *testing.T) {
	m, routes := sampleManifest()
	db := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "aaa-db",
		Title:               "AAA Database",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
	reg := plugin.NewRegistry()
	if err := reg.Register(&stubPlugin{manifest: db}); err != nil {
		t.Fatalf("register db: %v", err)
	}
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register shell: %v", err)
	}
	s := reg.Summaries()
	if got := []string{s[0].Name, s[1].Name}; got[0] != "sample" || got[1] != "aaa-db" {
		t.Fatalf("summary order = %v, want [sample aaa-db]", got)
	}
}

func TestValidateAcceptsAllLayouts(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	for _, layout := range []plugin.Layout{plugin.LayoutTabs, plugin.LayoutSidebarTree, plugin.LayoutDashboard} {
		m := plugin.Manifest{
			APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
			Category: plugin.CategoryOther, Layout: layout,
			SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		}
		routes := []plugin.Route{{ID: "x.list", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop}}
		if err := plugin.Validate(m, routes); err != nil {
			t.Fatalf("layout %q should be valid: %v", layout, err)
		}
	}
}

func TestValidateRejectsBadManifests(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	base := func() (plugin.Manifest, []plugin.Route) {
		return plugin.Manifest{
				APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
				Category: plugin.CategoryOther, Layout: plugin.LayoutTabs, SupportedTransports: []plugin.Transport{plugin.TransportDirect},
			}, []plugin.Route{
				{ID: "x.list", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop},
			}
	}

	tests := []struct {
		name string
		want string
		mut  func(*plugin.Manifest, *[]plugin.Route)
	}{
		{"unsupported api version", "APIVersion", func(m *plugin.Manifest, _ *[]plugin.Route) { m.APIVersion = 99 }},
		{"missing name", "Name is required", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "" }},
		{"missing category", "Category is required", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Category = "" }},
		{"unknown category", "not a built-in category", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Category = "weird" }},
		{"missing direct transport", "must include", func(m *plugin.Manifest, _ *[]plugin.Route) { m.SupportedTransports = nil }},
		{"agent without profile", "AgentProfile is required", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.SupportedTransports = []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent}
		}},
		{"duplicate route id", "duplicate route ID", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.list", Method: plugin.MethodGet, Permission: "p", Risk: plugin.RiskSafe, Handle: noop})
		}},
		{"route missing permission", "missing a Permission", func(_ *plugin.Manifest, r *[]plugin.Route) {
			(*r)[0].Permission = ""
		}},
		{"ws route missing stream", "missing a Stream", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.ws", Method: plugin.MethodWS, Permission: "p", Risk: plugin.RiskSafe})
		}},
		{"tab references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "t", Label: "T", Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ghost"}}}
		}},
		{"file browser config references unknown route", "uploadRouteId references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "files", Label: "Files", Panel: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"uploadRouteId": "ghost"}}}
		}},
		{"file browser upload route requires file input", "without a file input schema", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.upload", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Handle: noop})
			m.Tabs = []plugin.Tab{{Key: "files", Label: "Files", Panel: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"uploadRouteId": "x.upload"}}}
		}},
		{"form submit route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "form", Label: "Form", Panel: plugin.PanelForm, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"submitRouteId": "x.list"}}}
		}},
		{"kv write route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "kv", Label: "KV", Panel: plugin.PanelKV, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"writeRouteId": "x.list"}}}
		}},
		{"http client execute route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "http", Label: "HTTP", Panel: plugin.PanelHTTPClient, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"executeRouteId": "x.list"}}}
		}},
		{"remote desktop requires source", "missing a source", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "desktop", Label: "Desktop", Panel: plugin.PanelRemoteDesktop, Config: plugin.RemoteDesktopConfig{}.Map()}}
		}},
		{"remote desktop source must be stream", "invalid stream method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "desktop", Label: "Desktop", Panel: plugin.PanelRemoteDesktop, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.RemoteDesktopConfig{}.Map()}}
		}},
		{"remote desktop rejects stale engine selector", "no longer accepts remote desktop engine", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.desktop", Method: plugin.MethodWS, Permission: "x.desktop", Risk: plugin.RiskPrivileged, Stream: stream})
			m.Tabs = []plugin.Tab{{Key: "desktop", Label: "Desktop", Panel: plugin.PanelRemoteDesktop, Source: &plugin.DataSource{RouteID: "x.desktop"}, Config: map[string]any{"engine": "novnc"}}}
		}},
		{"action references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "ghost"}}
		}},
		{"action success references unknown tab", "onSuccess.selectTab", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "x.list", OnSuccess: &plugin.ActionSuccess{SelectTab: "ghost"}}}
		}},
		{"stream references non-ws route", "non-WS route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Streams = []plugin.Stream{{ID: "s", Kind: plugin.StreamLogs, RouteID: "x.list"}}
		}},
		{"resource references unknown action", "references unknown action", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Resources = []plugin.ResourceType{{Kind: "k", Title: "K", List: plugin.DataSource{RouteID: "x.list"}, ActionIDs: []string{"ghost"}}}
		}},
		{"credential ref missing selector", "missing Credential selector", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Auth"}}}
			m.Config.Groups[0].Fields = append(m.Config.Groups[0].Fields, plugin.Field{Key: "cred", Label: "Cred", Type: plugin.FieldCredentialRef})
		}},
		{"credential ref unknown kind", "unknown credential kind", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Auth"}}}
			m.Config.Groups[0].Fields = append(m.Config.Groups[0].Fields, plugin.Field{
				Key: "cred", Label: "Cred", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{"made_up"}},
			})
		}},
		{"credential kind duplicates existing catalog", "duplicate credential kind", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Auth"}}}
			m.CredentialKinds = []plugin.CredentialKindInfo{{Kind: plugin.CredentialDBPassword, Label: "Database password", SecretLabel: "Password"}}
			m.Config.Groups[0].Fields = append(m.Config.Groups[0].Fields, plugin.Field{
				Key: "cred", Label: "Cred", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{plugin.CredentialDBPassword}},
			})
		}},
		{"credential kind protocol list is derived", "must not declare CompatibleProtocols", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Auth"}}}
			m.CredentialKinds = []plugin.CredentialKindInfo{{
				Kind: "custom_password", Label: "Custom password", SecretLabel: "Password", CompatibleProtocols: []string{"x"},
			}}
			m.Config.Groups[0].Fields = append(m.Config.Groups[0].Fields, plugin.Field{
				Key: "cred", Label: "Cred", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{"custom_password"}},
			})
		}},
		{"credential kind declared but unused", "declared but not used", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.CredentialKinds = []plugin.CredentialKindInfo{{Kind: "custom_password", Label: "Custom password", SecretLabel: "Password"}}
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, routes := base()
			tc.mut(&m, &routes)
			err := plugin.Validate(m, routes)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.want)
			}
			if !contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateAcceptsGoodManifest(t *testing.T) {
	m, routes := sampleManifest()
	if err := plugin.Validate(m, routes); err != nil {
		t.Errorf("valid manifest rejected: %v", err)
	}
}

func TestValidateAcceptsRemoteDesktopConfig(t *testing.T) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "desktop",
		Title:      "Desktop",
		Category:   plugin.CategoryRemoteDesktop,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
		},
		Tabs: []plugin.Tab{{
			Key: "desktop", Label: "Desktop", Panel: plugin.PanelRemoteDesktop,
			Source: &plugin.DataSource{RouteID: "desktop.stream", Method: plugin.MethodWS},
			Config: plugin.RemoteDesktopConfig{Resize: true, Clipboard: true}.Map(),
		}},
		Streams: []plugin.Stream{{ID: "desktop.stream", Kind: plugin.StreamDesktop, RouteID: "desktop.stream"}},
	}
	routes := []plugin.Route{{
		ID: "desktop.stream", Method: plugin.MethodWS, Permission: "desktop.use",
		Risk: plugin.RiskPrivileged, Stream: stream,
	}}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("valid remote desktop manifest rejected: %v", err)
	}
}

func TestSpecializedPanelConfigMaps(t *testing.T) {
	kv := plugin.KVConfig{
		CreateRouteID: "redis.key.create", ReadRouteID: "redis.key.read", WriteRouteID: "redis.key.write",
		DeleteRouteID: "redis.key.delete", KeyParam: "key", Writable: true,
	}.Map()
	if kv["createRouteId"] != "redis.key.create" || kv["readRouteId"] != "redis.key.read" || kv["writable"] != true {
		t.Fatalf("kv config map unexpected: %#v", kv)
	}

	http := plugin.HTTPClientConfig{
		ExecuteRouteID: "http.execute", Methods: []string{"GET", "POST"},
		DefaultMethod: "GET", DefaultURL: "/health",
		DefaultHeaders: []plugin.HeaderDefault{{Key: "Accept", Value: "application/json"}},
	}.Map()
	if http["executeRouteId"] != "http.execute" || len(http["methods"].([]string)) != 2 {
		t.Fatalf("http config map unexpected: %#v", http)
	}

	graph := plugin.GraphConfig{Layout: plugin.GraphLayoutManual, FitView: true}.Map()
	if graph["layout"] != plugin.GraphLayoutManual || graph["fitView"] != true {
		t.Fatalf("graph config map unexpected: %#v", graph)
	}

	trace := plugin.TraceConfig{ServiceField: "process.serviceName"}.Map()
	if trace["serviceField"] != "process.serviceName" {
		t.Fatalf("trace config map unexpected: %#v", trace)
	}

	desktop := plugin.RemoteDesktopConfig{
		Resize:     true,
		Clipboard:  true,
		RepeaterID: "console-1",
	}.Map()
	if desktop["engine"] != nil || desktop["resize"] != true || desktop["repeaterID"] != "console-1" {
		t.Fatalf("remote desktop config map unexpected: %#v", desktop)
	}
}

func TestRegistryDerivesCredentialKindProtocolsFromSelectors(t *testing.T) {
	m, routes := sampleManifest()
	reg := plugin.NewRegistry()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}
	info, ok := reg.CredentialKindLookup(testCredentialSSHPrivateKey)
	if !ok {
		t.Fatal("ssh private key kind not registered")
	}
	if len(info.CompatibleProtocols) != 1 || info.CompatibleProtocols[0] != "ssh" {
		t.Fatalf("derived protocols = %+v, want [ssh]", info.CompatibleProtocols)
	}
	if !reg.CredentialKindSupportsProtocol(testCredentialSSHPrivateKey, "ssh") {
		t.Fatal("ssh private key should support ssh")
	}
	if reg.CredentialKindSupportsProtocol(testCredentialSSHPrivateKey, "postgres") {
		t.Fatal("ssh private key should not support postgres")
	}
}

func TestRegistryRejectsDuplicatePluginCredentialKind(t *testing.T) {
	m, routes := sampleManifest()
	reg := plugin.NewRegistry()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register first plugin: %v", err)
	}
	dup := m
	dup.Name = "duplicate"
	for i := range routes {
		routes[i].ID = "duplicate." + routes[i].ID
	}
	if err := reg.Register(&stubPlugin{manifest: dup, routes: routes}); err == nil || !contains(err.Error(), "duplicate credential kind") {
		t.Fatalf("duplicate credential kind error = %v", err)
	}
}

// recordingBase is a minimal manifest with a terminal + desktop stream, used to
// exercise recording-declaration validation.
func recordingBase() (plugin.Manifest, []plugin.Route) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "rec", Title: "Rec",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs, SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Streams: []plugin.Stream{
			{ID: "rec.shell", Kind: plugin.StreamTerminal, RouteID: "rec.shell"},
			{ID: "rec.screen", Kind: plugin.StreamDesktop, RouteID: "rec.screen"},
		},
	}
	routes := []plugin.Route{
		{ID: "rec.shell", Method: plugin.MethodWS, Permission: "rec.shell", Risk: plugin.RiskPrivileged, Stream: stream},
		{ID: "rec.screen", Method: plugin.MethodWS, Permission: "rec.screen", Risk: plugin.RiskPrivileged, Stream: stream},
	}
	return m, routes
}

func TestValidateRecordingAccepts(t *testing.T) {
	m, routes := recordingBase()
	m.Recording = []plugin.RecordingCapability{
		{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"rec.shell"}, Authoritative: true},
		{Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas}, StreamIDs: []string{"rec.screen"}},
	}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("valid terminal+desktop recording rejected: %v", err)
	}
}

func TestValidateRecordingRejects(t *testing.T) {
	tests := []struct {
		name string
		want string
		caps []plugin.RecordingCapability
	}{
		{"invalid class", "invalid class", []plugin.RecordingCapability{
			{Class: "weird", Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}},
		}},
		{"duplicate class", "duplicate recording class", []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"rec.shell"}},
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}},
		}},
		{"no formats", "declares no formats", []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, StreamIDs: []string{"rec.shell"}},
		}},
		{"no streams", "declares no streams", []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}},
		}},
		{"format/class mismatch", "does not support format", []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas}, StreamIDs: []string{"rec.shell"}},
		}},
		{"unknown stream", "references unknown stream", []plugin.RecordingCapability{
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"ghost"}},
		}},
		{"stream kind mismatch", "incompatible kind", []plugin.RecordingCapability{
			{Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas}, StreamIDs: []string{"rec.shell"}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, routes := recordingBase()
			m.Recording = tc.caps
			err := plugin.Validate(m, routes)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.want)
			}
			if !contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func newRC(params map[string]string, query url.Values, body string) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, nil, params, query, []byte(body))
}

func TestRequestContextBindTypedNoPanic(t *testing.T) {
	type scaleReq struct {
		Replicas int `json:"replicas" validate:"min=0,max=1000"`
	}

	// JSON number decodes to int — the case that panicked with map[string]any.
	rc := newRC(nil, nil, `{"replicas": 3}`)
	var req scaleReq
	if err := rc.Bind(&req); err != nil {
		t.Fatalf("bind valid: %v", err)
	}
	if req.Replicas != 3 {
		t.Errorf("replicas: want 3, got %d", req.Replicas)
	}

	// Out-of-range fails validation, does not panic.
	rc = newRC(nil, nil, `{"replicas": 99999}`)
	if err := rc.Bind(&scaleReq{}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("bind out-of-range: want ErrInvalidInput, got %v", err)
	}

	// Wrong type (string where int expected) fails cleanly, no panic.
	rc = newRC(nil, nil, `{"replicas": "lots"}`)
	if err := rc.Bind(&scaleReq{}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("bind wrong-type: want ErrInvalidInput, got %v", err)
	}

	// Empty body: validation still runs (missing required handled by tags).
	rc = newRC(nil, nil, ``)
	if err := rc.Bind(&scaleReq{}); err != nil {
		t.Errorf("bind empty body: %v", err)
	}
}

func TestSchemaRejectsUnknownFields(t *testing.T) {
	schema := plugin.Schema{Groups: []plugin.Group{{Name: "Input", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "upload", Label: "Upload", Type: plugin.FieldFile},
	}}}}
	if err := schema.ValidateValues(map[string]any{"name": "ok"}, nil); err != nil {
		t.Fatalf("valid schema values rejected: %v", err)
	}
	if err := schema.ValidateValues(map[string]any{"name": "ok", "extra": "no"}, nil); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unknown value field: want ErrInvalidInput, got %v", err)
	}
	if err := schema.ValidateValues(map[string]any{"name": "ok"}, map[string]bool{"ghost": true}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unknown upload field: want ErrInvalidInput, got %v", err)
	}
}

func TestRequestContextParamAndPage(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "25")
	q.Set("cursor", "abc")
	q.Set("filter", "web")
	q.Set("filter.state", "running")
	q.Set("sort", "-name")
	rc := newRC(map[string]string{"vmid": "101"}, q, "")

	if rc.Param("vmid") != "101" {
		t.Errorf("param vmid: got %q", rc.Param("vmid"))
	}
	page, err := rc.Page()
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if page.Limit != 25 || page.Cursor != "abc" {
		t.Errorf("page limit/cursor: %+v", page)
	}
	if page.Filter["q"] != "web" || page.Filter["state"] != "running" {
		t.Errorf("page filter: %+v", page.Filter)
	}
	if len(page.Sort) != 1 || page.Sort[0].Field != "name" || !page.Sort[0].Desc {
		t.Errorf("page sort: %+v", page.Sort)
	}
}

func TestPageLimitClamp(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "100000")
	page, _ := newRC(nil, q, "").Page()
	if page.Limit != plugin.MaxPageLimit {
		t.Errorf("limit clamp: want %d, got %d", plugin.MaxPageLimit, page.Limit)
	}
}
