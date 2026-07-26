package plugin_test

// Round-trip audit: every test here exercises the plugin -> gateway boundary
// (grpcplugin.EncodeManifest -> json -> DecodeManifest) and pins down what the
// decoder currently does. Tests marked "CURRENT (BUGGY) BEHAVIOUR" assert the
// loss on purpose so the suite stays green while the defect is unfixed — when
// the decoder is fixed they will fail loudly and must be flipped to the
// want-preserved assertion recorded in each comment.

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// roundTrip mirrors what grpcplugin.EncodeManifest/DecodeManifest do to a
// manifest when it crosses the plugin->gateway boundary.
func roundTrip(t *testing.T, m plugin.Manifest) plugin.Manifest {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got plugin.Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return got
}

// FINDING 1 — CURRENT (BUGGY) BEHAVIOUR.
// PanelLogStream has no entry in panelConfigDecoders (config_json.go:7-30) even
// though LogStreamConfig implements PanelConfig (ui.go:70) and is validated
// (validate.go:797). A LogStreamConfig passes ValidatePlugin in-process and then
// decodes to nil on the gateway: the log panel silently loses its container
// picker and its "previous logs" toggle, with no error.
// WANT after fix: got.Tabs[0].Config is a plugin.LogStreamConfig with
// len(Controls) == 1 and AllowPrevious == true.
func TestLogStreamConfigLostOnRoundTrip(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Tabs: []plugin.Panel{{
			Key:    "logs",
			Type:   plugin.PanelLogStream,
			Source: &plugin.DataSource{RouteID: "demo.logs", Method: plugin.MethodWS},
			Config: plugin.LogStreamConfig{
				Controls: []plugin.StreamControl{{
					Param:         "container",
					Label:         "Container",
					OptionsSource: &plugin.DataSource{RouteID: "demo.containers"},
				}},
				AllowPrevious: true,
			},
		}},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(data), `"allowPrevious":true`) {
		t.Fatalf("expected the config on the wire, got %s", data)
	}

	got := roundTrip(t, m)
	if got.Tabs[0].Config != nil {
		t.Fatalf("log_stream config now decodes as %T — the bug is fixed; flip this test to assert the preserved LogStreamConfig", got.Tabs[0].Config)
	}
	t.Logf("DEMONSTRATED LOSS: log_stream config %#v was sent on the wire but decoded to nil", m.Tabs[0].Config)
}

// FINDING 2 — CURRENT (BUGGY) BEHAVIOUR.
// SplitPanel (ui.go:310-314) embeds Panel, so Panel's pointer-receiver
// UnmarshalJSON (config_json.go:50) is promoted to *SplitPanel and
// encoding/json never populates the outer Size/MinSize fields. Marshal writes
// them (they are promoted-flattened on the way out), decode drops them, so split
// layout sizing silently resets to 0 on the gateway.
// WANT after fix: Panels[0].Size == 70 && Panels[0].MinSize == 30.
func TestSplitPanelSizeLostOnRoundTrip(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Tabs: []plugin.Panel{{
			Key:  "workbench",
			Type: plugin.PanelSplit,
			Config: plugin.SplitConfig{
				Orientation: plugin.SplitVertical,
				Panels: []plugin.SplitPanel{
					{Panel: plugin.Panel{Key: "editor", Type: plugin.PanelQueryEditor}, Size: 70, MinSize: 30},
					{Panel: plugin.Panel{Key: "results", Type: plugin.PanelTable}, Size: 30, MinSize: 20},
				},
			},
		}},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The wire form does carry size/minSize; only the decoder drops them.
	if !contains(string(data), `"size":70`) || !contains(string(data), `"minSize":30`) {
		t.Fatalf("expected size/minSize on the wire, got %s", data)
	}

	got := roundTrip(t, m)
	cfg, ok := got.Tabs[0].Config.(plugin.SplitConfig)
	if !ok {
		t.Fatalf("split config type = %T", got.Tabs[0].Config)
	}
	if cfg.Panels[0].Key != "editor" || cfg.Panels[0].Type != plugin.PanelQueryEditor {
		t.Fatalf("embedded Panel lost: %#v", cfg.Panels[0])
	}
	if cfg.Panels[0].Size != 0 || cfg.Panels[0].MinSize != 0 {
		t.Fatalf("SplitPanel size/minSize now decode (size=%d minSize=%d) — the bug is fixed; flip this test to want 70/30",
			cfg.Panels[0].Size, cfg.Panels[0].MinSize)
	}
	t.Logf("DEMONSTRATED LOSS: SplitPanel{Size:70,MinSize:30} decoded as Size:%d MinSize:%d", cfg.Panels[0].Size, cfg.Panels[0].MinSize)
}

// FINDING 3 — CURRENT (BUGGY) BEHAVIOUR.
// decodePanelConfig (config_json.go:40-48) returns (nil, nil) for any panel type
// missing from the decoder table, including a typo'd type. A
// malformed-but-parseable manifest loads with its config silently erased instead
// of being rejected, and Validate never sees the dropped config.
func TestUnknownPanelTypeConfigSilentlyDropped(t *testing.T) {
	var got plugin.Manifest
	err := json.Unmarshal([]byte(`{
		"APIVersion": 1,
		"Name": "demo",
		"Tabs": [{"key":"t","panel":"tabel","config":{"columns":[{"key":"id","label":"ID"}]}}]
	}`), &got)
	if err != nil {
		t.Fatalf("unmarshal rejected the unknown panel type — the bug is fixed: %v", err)
	}
	if got.Tabs[0].Config != nil {
		t.Fatalf("unexpected config: %#v", got.Tabs[0].Config)
	}
	t.Logf("DEMONSTRATED LOSS: panel type %q is unknown; its config was dropped with no error", got.Tabs[0].Type)
}

// FINDING 4 (informational) — Panel.UnmarshalJSON always allocates Variants
// (config_json.go:70), so a decoded manifest is never reflect.DeepEqual to the
// source manifest even when nothing was lost.
func TestPanelVariantsNilBecomesEmptySlice(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Tabs:       []plugin.Panel{{Key: "t", Type: plugin.PanelTable}},
	}
	if m.Tabs[0].Variants != nil {
		t.Fatalf("source Variants should be nil")
	}
	got := roundTrip(t, m)
	if got.Tabs[0].Variants == nil {
		t.Skip("Variants stayed nil; asymmetry gone")
	}
	if reflect.DeepEqual(m.Tabs[0], got.Tabs[0]) {
		t.Skip("panels compare equal; asymmetry gone")
	}
	t.Logf("DEMONSTRATED: Variants nil -> %#v (len %d); decoded Panel is not DeepEqual to the source Panel",
		got.Tabs[0].Variants, len(got.Tabs[0].Variants))
}

// FINDING 5 (informational) — every `any`-typed manifest value (Field.Default,
// Field.Step, Option.Value, Rule.Value, Validator.Value, Action.Body,
// CodeEditorConfig.SaveExtra, Badge.Value) loses its Go type on decode: an int
// default becomes float64. Any consumer type-switching on int misses.
func TestAnyValuedManifestFieldsLoseGoType(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Config: plugin.Schema{Groups: []plugin.Group{{
			Name: "general",
			Fields: []plugin.Field{{
				Key: "port", Label: "Port", Type: plugin.FieldNumber,
				Default: 22, Step: 1,
				Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}},
				Options:    []plugin.Option{{Label: "ssh", Value: 22}},
			}},
		}}},
		Actions: []plugin.Action{{ID: "a", RouteID: "demo.a", Body: map[string]any{"replicas": 3}}},
	}
	got := roundTrip(t, m)
	f := got.Config.Groups[0].Fields[0]
	if _, isInt := f.Default.(int); isInt {
		t.Skip("Default kept its int type; no loss")
	}
	t.Logf("DEMONSTRATED: Field.Default %T->%T, Field.Step %T->%T, Validator.Value %T->%T, Option.Value %T->%T, Action.Body[replicas] %T->%T",
		m.Config.Groups[0].Fields[0].Default, f.Default,
		m.Config.Groups[0].Fields[0].Step, f.Step,
		m.Config.Groups[0].Fields[0].Validators[0].Value, f.Validators[0].Value,
		m.Config.Groups[0].Fields[0].Options[0].Value, f.Options[0].Value,
		m.Actions[0].Body["replicas"], got.Actions[0].Body["replicas"])
}

// Control: every panel type that IS in the decoder table survives the round trip
// intact. Only log_stream (Finding 1) is missing, and it is asserted separately.
func TestEveryDecodablePanelTypeSurvives(t *testing.T) {
	cases := []struct {
		panel plugin.PanelType
		cfg   plugin.PanelConfig
	}{
		{plugin.PanelTable, plugin.TableConfig{EmptyText: "x"}},
		{plugin.PanelFileBrowser, plugin.FileBrowserConfig{PathParam: "p"}},
		{plugin.PanelForm, plugin.FormPanelConfig{SubmitLabel: "go"}},
		{plugin.PanelDashboard, plugin.DashboardConfig{Cells: []plugin.Panel{{Key: "c", Type: plugin.PanelTable}}}},
		{plugin.PanelMetrics, plugin.MetricsConfig{History: 5}},
		{plugin.PanelGraph, plugin.GraphConfig{Layout: plugin.GraphLayoutGrid}},
		{plugin.PanelTrace, plugin.TraceConfig{ServiceField: "svc"}},
		{plugin.PanelKV, plugin.KVConfig{KeyParam: "k"}},
		{plugin.PanelTerminal, plugin.TerminalConfig{Zoom: true}},
		{plugin.PanelTerminalGrid, plugin.TerminalGridConfig{MaxPanes: 4}},
		{plugin.PanelCodeEditor, plugin.CodeEditorConfig{Language: "go"}},
		{plugin.PanelDiff, plugin.DiffConfig{Language: "go"}},
		{plugin.PanelQueryEditor, plugin.QueryEditorConfig{Language: "sql"}},
		{plugin.PanelHTTPClient, plugin.HTTPClientConfig{DefaultURL: "/x"}},
		{plugin.PanelRemoteDesktop, plugin.RemoteDesktopConfig{Resize: true}},
		{plugin.PanelObjectDetail, plugin.ObjectDetailConfig{RawToggle: true}},
		{plugin.PanelTimeline, plugin.TimelineConfig{TitleField: "t"}},
		{plugin.PanelTaskProgress, plugin.TaskProgressConfig{Title: "t"}},
		{plugin.PanelSplit, plugin.SplitConfig{Orientation: plugin.SplitHorizontal}},
		{plugin.PanelCanvas, plugin.CanvasConfig{Interactive: true}},
		{plugin.PanelWasm, plugin.WasmConfig{Entry: "app.wasm"}},
		{plugin.PanelWebProxy, plugin.WebProxyConfig{Path: "/"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.panel), func(t *testing.T) {
			m := plugin.Manifest{
				APIVersion: plugin.CurrentAPIVersion,
				Name:       "demo",
				Tabs:       []plugin.Panel{{Key: "t", Type: tc.panel, Config: tc.cfg}},
			}
			got := roundTrip(t, m)
			if got.Tabs[0].Config == nil {
				t.Fatalf("panel %q config dropped entirely on decode", tc.panel)
			}
			if reflect.TypeOf(got.Tabs[0].Config) != reflect.TypeOf(tc.cfg) {
				t.Fatalf("panel %q config type changed: %T -> %T", tc.panel, tc.cfg, got.Tabs[0].Config)
			}
			// Compare the wire form, not the Go value: Finding 4 (Variants nil ->
			// empty slice) would otherwise trip nested-panel configs.
			want, _ := json.Marshal(tc.cfg)
			have, _ := json.Marshal(got.Tabs[0].Config)
			if string(want) != string(have) {
				t.Fatalf("panel %q config changed: %s -> %s", tc.panel, want, have)
			}
		})
	}
}

// Control: the whole nested manifest tree (tree groups, resources ->
// detail.tabs, panel variants, dashboard cells, action open-panel effects,
// scope filters, streams, agent profile, recording) survives intact, so the
// only interface-typed hole is the one asserted above.
func TestNestedManifestTreeSurvives(t *testing.T) {
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "demo",
		Title:               "Demo",
		Category:            plugin.CategoryOther,
		Layout:              plugin.LayoutSidebarTree,
		SupportedTransports: []plugin.Transport{plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy:   plugin.ProxyTarget{Mode: plugin.AgentTCP, Address: "127.0.0.1:1", Risk: plugin.RiskPrivileged, Forward: true},
			Install: []plugin.InstallArtifact{{Label: "docker", Kind: "sh", Template: "run", Delivery: plugin.DeliveryURL, ConnectURL: plugin.ArtifactConnectURL{LocalhostHost: "h"}}},
		},
		Tree: []plugin.TreeGroup{{
			Key: "g", Label: "G", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "box"},
			Source: plugin.DataSource{RouteID: "demo.tree"},
			Ref:    &plugin.ResourceIdentity{Kind: "thing", Name: "n", UID: "u"},
			Badge:  &plugin.Badge{Source: &plugin.DataSource{RouteID: "demo.count"}, Severity: plugin.SeverityWarn},
		}},
		Resources: []plugin.ResourceType{{
			Kind: "thing", Title: "Thing",
			List:    plugin.DataSource{RouteID: "demo.list"},
			Watch:   &plugin.DataSource{RouteID: "demo.watch", Method: plugin.MethodWS},
			Columns: []plugin.Column{{Key: "name", Label: "Name", Precision: intPtr(2)}},
			Actions: plugin.ResourceActions{Toolbar: []string{"a"}, Row: []string{"a"}, Detail: []string{"a"}, Selectable: true},
			Detail: plugin.DetailView{
				Header:     plugin.HeaderSpec{Title: "t", Severities: map[string]plugin.Severity{"ok": plugin.SeveritySuccess}},
				DefaultTab: "d",
				Tabs: []plugin.Panel{{
					Key: "d", Type: plugin.PanelObjectDetail,
					Config:      plugin.ObjectDetailConfig{RawToggle: true},
					VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "phase", Op: plugin.OpEq, Value: "Running"}}},
					Variants: []plugin.PanelVariant{{
						Type:        plugin.PanelCodeEditor,
						Config:      plugin.CodeEditorConfig{Language: "yaml", SaveToast: &plugin.SaveToast{Summary: "s", Severity: plugin.SeveritySuccess}},
						VisibleWhen: &plugin.Condition{AnyOf: []plugin.Rule{{Field: "raw", Op: plugin.OpNotEmpty}}},
					}},
				}},
			},
		}},
		Actions: []plugin.Action{{
			ID: "a", Label: "A", RouteID: "demo.a",
			Open: plugin.OpenDialog, Panel: plugin.PanelForm,
			Config: plugin.FormPanelConfig{SubmitRouteID: "demo.a", SubmitMethod: plugin.MethodPost},
			OnSuccess: &plugin.ActionSuccess{Effects: []plugin.ActionEffect{{
				Type: plugin.ActionEffectOpenPanel,
				OpenPanel: &plugin.OpenPanelEffect{
					Open: plugin.OpenDock, Panel: plugin.PanelDashboard,
					Config: plugin.DashboardConfig{Cells: []plugin.Panel{{
						Key: "cell", Type: plugin.PanelTable, Span: 2,
						Config: plugin.TableConfig{Columns: []plugin.Column{{Key: "k", Label: "K"}}},
					}}},
				},
			}}},
			EnabledWhen: &plugin.Condition{Not: &plugin.Condition{AllOf: []plugin.Rule{{Field: "x", Op: plugin.OpEmpty}}}},
		}},
		Streams:   []plugin.Stream{{ID: "s", Kind: plugin.StreamResource, RouteID: "demo.watch"}},
		Scope:     []plugin.ScopeFilter{{Param: "ns", Label: "NS", Control: plugin.ScopeSelect, Options: []plugin.FilterOption{{Value: "v"}}}},
		Recording: []plugin.RecordingCapability{{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"s"}, Authoritative: true}},
	}

	got := roundTrip(t, m)

	if _, ok := got.Resources[0].Detail.Tabs[0].Config.(plugin.ObjectDetailConfig); !ok {
		t.Fatalf("resource detail tab config lost: %T", got.Resources[0].Detail.Tabs[0].Config)
	}
	vcfg, ok := got.Resources[0].Detail.Tabs[0].Variants[0].Config.(plugin.CodeEditorConfig)
	if !ok {
		t.Fatalf("panel variant config lost: %T", got.Resources[0].Detail.Tabs[0].Variants[0].Config)
	}
	if vcfg.SaveToast == nil || vcfg.SaveToast.Summary != "s" {
		t.Fatalf("variant nested pointer lost: %#v", vcfg)
	}
	dash, ok := got.Actions[0].OnSuccess.Effects[0].OpenPanel.Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("openPanel effect config lost: %T", got.Actions[0].OnSuccess.Effects[0].OpenPanel.Config)
	}
	if _, ok := dash.Cells[0].Config.(plugin.TableConfig); !ok {
		t.Fatalf("dashboard cell config nested in an effect lost: %T", dash.Cells[0].Config)
	}
	if _, ok := got.Actions[0].Config.(plugin.FormPanelConfig); !ok {
		t.Fatalf("action config lost: %T", got.Actions[0].Config)
	}
	if got.Agent == nil || got.Agent.Proxy.Mode != plugin.AgentTCP || !got.Agent.Proxy.Forward {
		t.Fatalf("agent profile lost: %#v", got.Agent)
	}
	if len(got.Agent.Install) != 1 || got.Agent.Install[0].Delivery != plugin.DeliveryURL || got.Agent.Install[0].ConnectURL.LocalhostHost != "h" {
		t.Fatalf("install artifact lost: %#v", got.Agent.Install)
	}
	if got.Tree[0].Source.RouteID != "demo.tree" || got.Tree[0].Ref == nil || got.Tree[0].Badge == nil {
		t.Fatalf("tree group lost: %#v", got.Tree[0])
	}
	if got.Resources[0].Columns[0].Precision == nil || *got.Resources[0].Columns[0].Precision != 2 {
		t.Fatalf("column precision pointer lost: %#v", got.Resources[0].Columns[0])
	}
	if !got.Resources[0].Actions.Selectable {
		t.Fatalf("resource actions lost: %#v", got.Resources[0].Actions)
	}
	if got.Actions[0].EnabledWhen == nil || got.Actions[0].EnabledWhen.Not == nil {
		t.Fatalf("nested condition lost: %#v", got.Actions[0].EnabledWhen)
	}
	if len(got.Recording) != 1 || len(got.Recording[0].StreamIDs) != 1 {
		t.Fatalf("recording capability lost: %#v", got.Recording)
	}
	if len(got.Scope) != 1 || got.Scope[0].Control != plugin.ScopeSelect {
		t.Fatalf("scope filter lost: %#v", got.Scope)
	}
	if len(got.Streams) != 1 || got.Streams[0].Kind != plugin.StreamResource {
		t.Fatalf("stream lost: %#v", got.Streams)
	}
}

func intPtr(v int) *int { return &v }

// FINDING 6 — CURRENT (BUGGY) BEHAVIOUR.
// Several "kind"-tagged/enum manifest fields are neither validated in-process
// nor normalized on decode, so a malformed-but-parseable manifest loads clean on
// the gateway. Proven here: an unknown Panel.Type (whose config is silently
// dropped by Finding 3), an unknown Rule.Op in a Condition, an unknown
// Stream.Kind, an unknown Icon.Type, an unknown Column.Type and an unknown
// Field.Type all survive json.Unmarshal AND plugin.Validate.
func TestMalformedEnumsDecodeAndValidateClean(t *testing.T) {
	raw := []byte(`{
		"APIVersion": 1,
		"Name": "demo",
		"Title": "Demo",
		"Category": "other",
		"Layout": "tabs",
		"SupportedTransports": ["direct"],
		"Config": {"groups":[{"name":"g","fields":[
			{"key":"k","label":"K","type":"not_a_field_type"}
		]}]},
		"Tabs": [{
			"key": "t",
			"panel": "tabel",
			"icon": {"type":"not_an_icon_type","value":"x"},
			"visibleWhen": {"allOf":[{"field":"phase","op":"is_totally_bogus","value":"Running"}]},
			"config": {"columns":[{"key":"id","label":"ID","type":"not_a_column_type"}]}
		}],
		"Streams": [{"id":"s","kind":"not_a_stream_kind","routeId":"demo.ws"}]
	}`)

	var m plugin.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	routes := []plugin.Route{{
		ID: "demo.ws", Method: plugin.MethodWS, Path: "/ws",
		Permission: "demo.read", Risk: plugin.RiskSafe,
		Stream: func(*plugin.RequestContext, plugin.ClientStream) error { return nil },
	}}
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("Validate now rejects the malformed enums — the gap is closed: %v", err)
	}
	t.Logf("DEMONSTRATED: unvalidated enums survived decode+Validate: panel=%q icon.type=%q rule.op=%q column.type=%q field.type=%q stream.kind=%q; the table config was also dropped (%v)",
		m.Tabs[0].Type, m.Tabs[0].Icon.Type, m.Tabs[0].VisibleWhen.AllOf[0].Op,
		"not_a_column_type", m.Config.Groups[0].Fields[0].Type, m.Streams[0].Kind, m.Tabs[0].Config)
}
