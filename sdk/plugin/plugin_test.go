package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

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

func TestValidateCompositeSchemaFields(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	base := func(input *plugin.Schema) ([]plugin.Route, plugin.Manifest) {
		m := plugin.Manifest{
			APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
			Category: plugin.CategoryOther, Layout: plugin.LayoutTabs,
			SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		}
		routes := []plugin.Route{{ID: "x.create", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Input: input, Handle: noop}}
		return routes, m
	}

	columns := &plugin.Schema{Groups: []plugin.Group{{Name: "Table", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "columns", Label: "Columns", Type: plugin.FieldArray, Item: &plugin.Field{
			Type: plugin.FieldObject, Fields: []plugin.Field{
				{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
				{Key: "type", Label: "Type", Type: plugin.FieldSelect, Options: []plugin.Option{{Label: "Int", Value: "INT"}}},
				{Key: "nullable", Label: "Nullable", Type: plugin.FieldToggle},
			},
		}},
	}}}}
	routes, m := base(columns)
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("valid composite schema rejected: %v", err)
	}

	bad := map[string]*plugin.Schema{
		"object without fields": {Groups: []plugin.Group{{Name: "G", Fields: []plugin.Field{{Key: "o", Type: plugin.FieldObject}}}}},
		"array without item":    {Groups: []plugin.Group{{Name: "G", Fields: []plugin.Field{{Key: "a", Type: plugin.FieldArray}}}}},
		"array min over max":    {Groups: []plugin.Group{{Name: "G", Fields: []plugin.Field{{Key: "a", Type: plugin.FieldArray, MinItems: 3, MaxItems: 1, Item: &plugin.Field{Type: plugin.FieldText}}}}}},
	}
	for name, input := range bad {
		routes, m := base(input)
		if err := plugin.Validate(m, routes); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
}

func TestDashboardConfigMapAndPanelValidate(t *testing.T) {
	cfg := plugin.DashboardConfig{Cells: []plugin.Panel{
		{Key: "a", Label: "A", Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "x.overview"}, Span: 2},
		{Key: "b", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "x.list"}},
	}}
	if len(cfg.Cells) != 2 || cfg.Cells[0].Span != 2 {
		t.Fatalf("DashboardConfig cells = %#v", cfg.Cells)
	}

	// A dashboard-panel tab carries its cells in config and needs no tab source.
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs:                []plugin.Panel{{Key: "overview", Label: "Overview", Type: plugin.PanelDashboard, Config: cfg}},
	}
	routes := []plugin.Route{
		{ID: "x.overview", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop},
		{ID: "x.list", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop},
	}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("dashboard-panel tab should validate: %v", err)
	}
}

func TestMetricsConfigMap(t *testing.T) {
	full := plugin.MetricsConfig{
		Stats:   []plugin.MetricStat{{Key: "conns", Label: "Connections"}},
		Gauges:  []plugin.MetricGauge{{Key: "cpu", Label: "CPU", Unit: "%", Max: 100}},
		Series:  []plugin.MetricSeries{{Key: "cpu", Label: "CPU"}},
		History: 120,
	}
	if len(full.Stats) == 0 || len(full.Gauges) == 0 || len(full.Series) == 0 || full.History != 120 {
		t.Fatalf("MetricsConfig = %#v", full)
	}
}

func TestDockActionRequiresPanel(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	routes := []plugin.Route{{ID: "x.logs", Method: plugin.MethodWS, Permission: "x.read", Risk: plugin.RiskSafe, Stream: func(*plugin.RequestContext, plugin.ClientStream) error { return nil }}, {ID: "x.list", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop}}
	base := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
	bad := base
	bad.Actions = []plugin.Action{{ID: "a", Label: "Logs", RouteID: "x.logs", Open: plugin.OpenDock}}
	if err := plugin.Validate(bad, routes); err == nil {
		t.Fatal("dock action without a panel should be rejected")
	}
	ok := base
	ok.Actions = []plugin.Action{
		{ID: "a", Label: "Logs", RouteID: "x.logs", Open: plugin.OpenDock, Panel: plugin.PanelLogStream},
		{ID: "b", Label: "Peek", RouteID: "x.logs", Open: plugin.OpenDialog, Panel: plugin.PanelLogStream},
	}
	if err := plugin.Validate(ok, routes); err != nil {
		t.Fatalf("dock/dialog actions with a panel should validate: %v", err)
	}
}

func TestValidateRejectsBadManifests(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
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
		{"name with uppercase", "Name \"Bad\" is invalid", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "Bad" }},
		{"name with dot", "Name \"bad.plugin\" is invalid", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "bad.plugin" }},
		{"name with slash", "Name \"bad/plugin\" is invalid", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "bad/plugin" }},
		{"name with leading digit", "Name \"1bad\" is invalid", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "1bad" }},
		{"missing category", "Category is required", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Category = "" }},
		{"unknown category", "not a built-in category", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Category = "weird" }},
		{"missing direct transport", "must include", func(m *plugin.Manifest, _ *[]plugin.Route) { m.SupportedTransports = nil }},
		{"agent without profile", "AgentProfile is required", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.SupportedTransports = []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent}
		}},
		{"duplicate route id", "duplicate route ID", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.list", Method: plugin.MethodGet, Permission: "p", Risk: plugin.RiskSafe, Handle: noop})
		}},
		{"route outside plugin namespace", "must be namespaced under plugin", func(_ *plugin.Manifest, r *[]plugin.Route) {
			(*r)[0].ID = "other.list"
		}},
		{"route missing permission", "missing a Permission", func(_ *plugin.Manifest, r *[]plugin.Route) {
			(*r)[0].Permission = ""
		}},
		{"ws route missing stream", "missing a Stream", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.ws", Method: plugin.MethodWS, Permission: "p", Risk: plugin.RiskSafe})
		}},
		{"tab references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "t", Label: "T", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ghost"}}}
		}},
		{"tab source method must match route", "declares method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "t", Label: "T", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "x.list", Method: plugin.MethodPost}}}
		}},
		{"query editor source must be stream", "invalid stream method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "query", Label: "Query", Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "x.list"}}}
		}},
		{"table insert source must be write route", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "table", Label: "Table", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.TableConfig{
				Editable: true,
				Insert:   &plugin.DataSource{RouteID: "x.list"},
			}}}
		}},
		{"table watch source must be stream", "invalid stream method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "table", Label: "Table", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.TableConfig{
				Watch: &plugin.DataSource{RouteID: "x.list"},
			}}}
		}},
		{"dashboard cell source must validate", "cell \"logs\" source references route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "overview", Label: "Overview", Type: plugin.PanelDashboard, Config: plugin.DashboardConfig{Cells: []plugin.Panel{{
				Key: "logs", Label: "Logs", Type: plugin.PanelLogStream, Source: &plugin.DataSource{RouteID: "x.list"},
			}}}}}
		}},
		{"file browser config references unknown route", "uploadRouteId references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "files", Label: "Files", Type: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.FileBrowserConfig{UploadRouteID: "ghost"}}}
		}},
		{"file browser upload route requires file input", "without a file input schema", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.upload", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Handle: noop})
			m.Tabs = []plugin.Panel{{Key: "files", Label: "Files", Type: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.FileBrowserConfig{UploadRouteID: "x.upload"}}}
		}},
		{"form submit route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "form", Label: "Form", Type: plugin.PanelForm, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.FormPanelConfig{SubmitRouteID: "x.list"}}}
		}},
		{"form submit method must be write method", "submitMethod has invalid write method", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.write", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Handle: noop})
			m.Tabs = []plugin.Panel{{Key: "form", Label: "Form", Type: plugin.PanelForm, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.FormPanelConfig{SubmitRouteID: "x.write", SubmitMethod: plugin.MethodGet}}}
		}},
		{"code editor save method must be write method", "saveMethod has invalid write method", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.write", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Handle: noop})
			m.Tabs = []plugin.Panel{{Key: "editor", Label: "Editor", Type: plugin.PanelCodeEditor, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.CodeEditorConfig{SaveRouteID: "x.write", SaveMethod: plugin.MethodWS}}}
		}},
		{"kv write route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "kv", Label: "KV", Type: plugin.PanelKV, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.KVConfig{WriteRouteID: "x.list"}}}
		}},
		{"http client execute route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "http", Label: "HTTP", Type: plugin.PanelHTTPClient, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.HTTPClientConfig{ExecuteRouteID: "x.list"}}}
		}},
		{"remote desktop requires source", "missing a source", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "desktop", Label: "Desktop", Type: plugin.PanelRemoteDesktop, Config: plugin.RemoteDesktopConfig{}}}
		}},
		{"remote desktop source must be stream", "invalid stream method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Panel{{Key: "desktop", Label: "Desktop", Type: plugin.PanelRemoteDesktop, Source: &plugin.DataSource{RouteID: "x.list"}, Config: plugin.RemoteDesktopConfig{}}}
		}},
		{"action references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "ghost"}}
		}},
		{"action success references unknown tab", "onSuccess.selectTab", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "x.list", OnSuccess: &plugin.ActionSuccess{SelectTab: "ghost"}}}
		}},
		{"action navigate unknown target", "is not a known target", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "x.list", OnSuccess: &plugin.ActionSuccess{Navigate: "sideways"}}}
		}},
		{"header action references unknown action", "headerAction \"ghost\" references unknown action", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.HeaderActions = []string{"ghost"}
		}},
		{"scope filter missing label", "is missing a label", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", OptionsSource: &plugin.DataSource{RouteID: "x.list"}}}
		}},
		{"scope select without choices", "has no choices", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", Label: "Namespace"}}
		}},
		{"scope optionsSource unknown route", "optionsSource references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", Label: "Namespace", OptionsSource: &plugin.DataSource{RouteID: "ghost"}}}
		}},
		{"scope watchSource unknown route", "watchSource references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", Label: "Namespace", OptionsSource: &plugin.DataSource{RouteID: "x.list"}, WatchSource: &plugin.DataSource{RouteID: "ghost"}}}
		}},
		{"scope watchSource must be websocket", "watchSource route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", Label: "Namespace", OptionsSource: &plugin.DataSource{RouteID: "x.list"}, WatchSource: &plugin.DataSource{RouteID: "x.list"}}}
		}},
		{"scope multiselect without choices", "has no choices", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "ns", Label: "Namespace", Control: plugin.ScopeMultiSelect}}
		}},
		{"scope toggle without option", "declares no option", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Scope = []plugin.ScopeFilter{{Param: "sys", Label: "System", Control: plugin.ScopeToggle}}
		}},
		{"action panel config references unknown save route", "saveRouteId references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "x.list", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor, Config: plugin.CodeEditorConfig{SaveRouteID: "ghost"}}}
		}},
		{"stream references non-ws route", "non-WS route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Streams = []plugin.Stream{{ID: "s", Kind: plugin.StreamLogs, RouteID: "x.list"}}
		}},
		{"resource references unknown action", "references unknown action", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Resources = []plugin.ResourceType{{Kind: "k", Title: "K", List: plugin.DataSource{RouteID: "x.list"}, Actions: plugin.ResourceActions{Detail: []string{"ghost"}}}}
		}},
		{"detail default tab references unknown tab", "defaultTab references unknown tab", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Resources = []plugin.ResourceType{{
				Kind: "k", Title: "K", List: plugin.DataSource{RouteID: "x.list"},
				Detail: plugin.DetailView{DefaultTab: "ghost", Tabs: []plugin.Panel{{Key: "overview", Label: "Overview", Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "x.list"}}}},
			}}
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
		Tabs: []plugin.Panel{{
			Key: "desktop", Label: "Desktop", Type: plugin.PanelRemoteDesktop,
			Source: &plugin.DataSource{RouteID: "desktop.stream", Method: plugin.MethodWS},
			Config: plugin.RemoteDesktopConfig{Resize: true, Clipboard: true},
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

func TestValidateAcceptsSplitAndTaskProgressPanels(t *testing.T) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "task",
		Title:      "Task",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
		},
		Tabs: []plugin.Panel{{
			Key:   "work",
			Label: "Work",
			Type:  plugin.PanelSplit,
			Config: plugin.SplitConfig{
				Orientation: plugin.SplitHorizontal,
				Panels: []plugin.SplitPanel{
					{
						Panel: plugin.Panel{
							Key: "rows", Label: "Rows", Type: plugin.PanelTable,
							Source: &plugin.DataSource{RouteID: "task.rows"},
						},
						Size: 40,
					},
					{
						Panel: plugin.Panel{
							Key: "run", Label: "Run", Type: plugin.PanelTaskProgress,
							Source: &plugin.DataSource{RouteID: "task.run", Method: plugin.MethodWS},
							Config: plugin.TaskProgressConfig{
								CancelRouteID: "task.cancel",
								RetryRouteID:  "task.retry",
							},
						},
						Size: 60,
					},
				},
			},
		}},
		Streams: []plugin.Stream{{ID: "task.run", Kind: plugin.StreamTask, RouteID: "task.run"}},
	}
	routes := []plugin.Route{
		{ID: "task.rows", Method: plugin.MethodGet, Permission: "task.read", Risk: plugin.RiskSafe, Handle: noop},
		{ID: "task.run", Method: plugin.MethodWS, Permission: "task.read", Risk: plugin.RiskSafe, Stream: stream},
		{ID: "task.cancel", Method: plugin.MethodPost, Permission: "task.write", Risk: plugin.RiskWrite, Handle: noop},
		{ID: "task.retry", Method: plugin.MethodPost, Permission: "task.write", Risk: plugin.RiskWrite, Handle: noop},
	}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("valid split/task manifest rejected: %v", err)
	}
}

func TestValidateAcceptsWasmPanel(t *testing.T) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "wasm",
		Title:      "WASM",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
		},
		Tabs: []plugin.Panel{{
			Key:   "app",
			Label: "App",
			Type:  plugin.PanelWasm,
			Config: plugin.WasmConfig{
				Entry:   "app.wasm",
				Runtime: plugin.WasmRuntimeGo,
				Boot:    plugin.WasmBoot{Scripts: []string{"wasm_exec.js"}},
				Assets: []plugin.WasmAsset{
					{Path: "wasm_exec.js", MIME: "text/javascript", Source: plugin.DataSource{RouteID: "wasm.asset", Params: map[string]string{"path": "wasm_exec.js"}}},
					{Path: "app.wasm", MIME: "application/wasm", Source: plugin.DataSource{RouteID: "wasm.asset", Params: map[string]string{"path": "app.wasm"}}},
				},
				Bridge: plugin.WasmBridge{
					Routes:  []plugin.WasmBridgeRoute{{RouteID: "wasm.state", Method: plugin.MethodGet}},
					Streams: []plugin.WasmBridgeStream{{RouteID: "wasm.events"}},
				},
			},
		}},
	}
	routes := []plugin.Route{
		{ID: "wasm.asset", Method: plugin.MethodGet, Permission: "wasm.read", Risk: plugin.RiskSafe, Handle: noop},
		{ID: "wasm.state", Method: plugin.MethodGet, Permission: "wasm.read", Risk: plugin.RiskSafe, Handle: noop},
		{ID: "wasm.events", Method: plugin.MethodWS, Permission: "wasm.read", Risk: plugin.RiskSafe, Stream: stream},
	}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("valid wasm manifest rejected: %v", err)
	}
}

func TestValidateRejectsBadCanvasPanel(t *testing.T) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	base := func() (plugin.Manifest, []plugin.Route) {
		return plugin.Manifest{
				APIVersion: plugin.CurrentAPIVersion,
				Name:       "canvas",
				Title:      "Canvas",
				Category:   plugin.CategoryOther,
				Layout:     plugin.LayoutTabs,
				SupportedTransports: []plugin.Transport{
					plugin.TransportDirect,
				},
				Tabs: []plugin.Panel{{
					Key:    "surface",
					Label:  "Surface",
					Type:   plugin.PanelCanvas,
					Source: &plugin.DataSource{RouteID: "canvas.stream", Method: plugin.MethodWS},
					Config: plugin.CanvasConfig{},
				}},
				Streams: []plugin.Stream{{ID: "canvas.stream", Kind: plugin.StreamCanvas, RouteID: "canvas.stream"}},
			}, []plugin.Route{
				{ID: "canvas.stream", Method: plugin.MethodWS, Permission: "canvas.read", Risk: plugin.RiskSafe, Stream: stream},
			}
	}
	tests := []struct {
		name string
		want string
		mut  func(*plugin.CanvasConfig)
	}{
		{"scale mode unsupported", "scaleMode", func(c *plugin.CanvasConfig) {
			c.ScaleMode = plugin.CanvasScaleMode("cover")
		}},
		{"partial dimensions rejected", "width and height must be declared together", func(c *plugin.CanvasConfig) {
			c.Width = 1280
		}},
		{"fit dimensions required", "scaleMode fit requires positive width and height", func(c *plugin.CanvasConfig) {
			c.ScaleMode = plugin.CanvasScaleFit
		}},
		{"scroll dimensions required", "scaleMode scroll requires positive width and height", func(c *plugin.CanvasConfig) {
			c.ScaleMode = plugin.CanvasScaleScroll
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, routes := base()
			cfg := m.Tabs[0].Config.(plugin.CanvasConfig)
			tc.mut(&cfg)
			m.Tabs[0].Config = cfg
			err := plugin.Validate(m, routes)
			if err == nil || !contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateRejectsBadWasmPanel(t *testing.T) {
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	base := func() (plugin.Manifest, []plugin.Route) {
		return plugin.Manifest{
				APIVersion: plugin.CurrentAPIVersion,
				Name:       "wasm",
				Title:      "WASM",
				Category:   plugin.CategoryOther,
				Layout:     plugin.LayoutTabs,
				SupportedTransports: []plugin.Transport{
					plugin.TransportDirect,
				},
				Tabs: []plugin.Panel{{
					Key:   "app",
					Label: "App",
					Type:  plugin.PanelWasm,
					Config: plugin.WasmConfig{
						Entry: "app.wasm",
						Assets: []plugin.WasmAsset{
							{Path: "app.wasm", Source: plugin.DataSource{RouteID: "wasm.asset"}},
						},
						Bridge: plugin.WasmBridge{
							Routes:  []plugin.WasmBridgeRoute{{RouteID: "wasm.state", Method: plugin.MethodGet}},
							Streams: []plugin.WasmBridgeStream{{RouteID: "wasm.events"}},
						},
					},
				}},
			}, []plugin.Route{
				{ID: "wasm.asset", Method: plugin.MethodGet, Permission: "wasm.read", Risk: plugin.RiskSafe, Handle: noop},
				{ID: "wasm.state", Method: plugin.MethodGet, Permission: "wasm.read", Risk: plugin.RiskSafe, Handle: noop},
				{ID: "wasm.events", Method: plugin.MethodWS, Permission: "wasm.read", Risk: plugin.RiskSafe, Stream: stream},
			}
	}
	tests := []struct {
		name string
		want string
		mut  func(*plugin.WasmConfig, *[]plugin.Route)
	}{
		{"entry not declared", "entry", func(c *plugin.WasmConfig, _ *[]plugin.Route) { c.Entry = "missing.wasm" }},
		{"entry required", "entry is required", func(c *plugin.WasmConfig, _ *[]plugin.Route) { c.Entry = "" }},
		{"runtime unsupported", "runtime", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Runtime = plugin.WasmRuntime("native")
		}},
		{"scale mode unsupported", "scaleMode", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.ScaleMode = plugin.WasmScaleMode("cover")
		}},
		{"partial dimensions rejected", "width and height must be declared together", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Width = 1280
		}},
		{"fit dimensions required", "scaleMode fit requires width and height", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.ScaleMode = plugin.WasmScaleFit
		}},
		{"boot script not declared", "boot.scripts", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Boot.Scripts = []string{"missing.js"}
		}},
		{"asset path required", "assets[0].path is required", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Entry = ""
			c.Assets[0].Path = ""
		}},
		{"asset path duplicated", "duplicated", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Assets = append(c.Assets, c.Assets[0])
		}},
		{"asset source unknown", "assets[0].source references unknown route", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Assets[0].Source.RouteID = "ghost"
		}},
		{"bridge route wrong method", "declares method", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Bridge.Routes[0].Method = plugin.MethodPost
		}},
		{"bridge route cannot be stream", "references stream route", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Bridge.Routes[0].RouteID = "wasm.events"
		}},
		{"bridge stream must be ws", "invalid stream method", func(c *plugin.WasmConfig, _ *[]plugin.Route) {
			c.Bridge.Streams[0].RouteID = "wasm.state"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, routes := base()
			cfg := m.Tabs[0].Config.(plugin.WasmConfig)
			tc.mut(&cfg, &routes)
			m.Tabs[0].Config = cfg
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

// TestPanelConfigWireFormat locks the JSON the browser receives for a panel
// config: typed structs serialize to the same camelCase keys the renderer reads,
// and zero-value fields are omitted.
func TestPanelConfigWireFormat(t *testing.T) {
	files := plugin.FileBrowserConfig{
		ReadRouteID: "sftp.read", Writable: true, UploadFieldName: "files",
	}
	b, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["readRouteId"] != "sftp.read" || got["writable"] != true || got["uploadFieldName"] != "files" {
		t.Fatalf("file browser wire format unexpected: %s", b)
	}
	if _, ok := got["downloadRouteId"]; ok {
		t.Fatalf("zero-value fields should be omitted: %s", b)
	}

	// A disabled terminal config serializes to an empty object (all omitempty).
	if b, _ := json.Marshal(plugin.TerminalConfig{}); string(b) != "{}" {
		t.Fatalf("empty terminal config = %s, want {}", b)
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
	return plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, nil, params, query, []byte(body))
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
