package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const testCredentialPrivateKey plugin.CredentialKind = "sample_private_key"

// sampleManifest exercises every projection-relevant shape: structured icons
// (incl. svg), a schema with credential_ref + condition + validators, a WS tab
// source, a tree group, a resource with detail tabs + actions, and a stream.
func sampleManifest() (plugin.Manifest, []plugin.Route) {
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "sample",
		Version:             "0.1.0",
		Title:               "Sample",
		Description:         "A representative plugin used by the golden test.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: "<svg viewBox=\"0 0 1 1\"></svg>"},
		Category:            plugin.CategoryShell,
		Capabilities:        []plugin.Capability{"terminal", "filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		CredentialKinds: []plugin.CredentialKindInfo{{
			Kind: testCredentialPrivateKey, Label: "Sample private key",
			Fields: []plugin.Field{
				plugin.CredentialPublicField(plugin.Field{Key: "username", Label: "Username", Type: plugin.FieldText}),
				plugin.CredentialSecretField(plugin.Field{Key: "private_key", Label: "Private key", Type: plugin.FieldTextarea}),
			},
		}},
		Layout: plugin.LayoutSidebarTree,
		Config: plugin.Schema{Groups: []plugin.Group{{
			Name: "Basic",
			Fields: []plugin.Field{
				{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1"},
				{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: 22, Validators: []plugin.Validator{
					{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535},
				}},
				{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &plugin.Condition{
					AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}},
				}},
				{Key: "credential_id", Label: "Credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
					Kind: testCredentialPrivateKey, Protocols: []string{"ssh"},
				}},
			},
		}}},
		Tabs: []plugin.Panel{{
			Key: "terminal", Label: "Terminal", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "terminal"},
			Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "sample.shell", Method: plugin.MethodWS},
		}, {
			Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "folder"},
			Type: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "sample.files.list", Params: map[string]string{"path": "/"}},
			Config: plugin.FileBrowserConfig{
				PathParam:       "path",
				ReadRouteID:     "sample.files.read",
				DownloadRouteID: "sample.files.download",
				UploadRouteID:   "sample.files.upload",
				MkdirRouteID:    "sample.files.mkdir",
				RenameRouteID:   "sample.files.rename",
				DeleteRouteID:   "sample.files.delete",
				Writable:        true,
				MultipleUpload:  true,
			},
		}},
		Tree: []plugin.TreeGroup{{
			Key: "containers", Label: "Containers", Source: plugin.DataSource{RouteID: "sample.list"}, ResourceKind: "container",
		}},
		Resources: []plugin.ResourceType{{
			Kind: "container", Title: "Containers", List: plugin.DataSource{RouteID: "sample.list"},
			Columns: []plugin.Column{{Key: "name", Label: "Name", Sortable: true, Type: plugin.ColumnText}}, Actions: plugin.ResourceActions{Detail: []string{"sample.start"}}, Detail: plugin.DetailView{
				Header:     plugin.HeaderSpec{Title: "Container"},
				DefaultTab: "editor",
				Tabs: []plugin.Panel{
					{Key: "logs", Label: "Logs", Type: plugin.PanelLogStream, Source: &plugin.DataSource{RouteID: "sample.logs", Method: plugin.MethodWS}},
					{Key: "summary", Label: "Summary", Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "sample.public"}, Config: plugin.ObjectDetailConfig{
						Sections: []plugin.ObjectDetailSection{{Title: "Usage", Fields: []plugin.ObjectDetailField{
							{Key: "memoryPct", Label: "Memory", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{
								PercentKey: "memoryPct",
								UsedKey:    "memoryUsed",
								TotalKey:   "memoryTotal",
								UsedType:   plugin.ColumnBytes,
								TotalType:  plugin.ColumnBytes,
								WarnAt:     80,
								CriticalAt: 95,
							}},
						}}},
					}},
					{Key: "config", Label: "Config", Type: plugin.PanelForm, Source: &plugin.DataSource{RouteID: "sample.form"}, Config: plugin.FormPanelConfig{
						SubmitRouteID: "sample.form.save",
						SubmitMethod:  plugin.MethodPatch,
						SubmitLabel:   "Apply",
						Params:        map[string]string{"id": "${resource.uid}"},
					}},
					{Key: "editor", Label: "Editor", Type: plugin.PanelCodeEditor, Source: &plugin.DataSource{RouteID: "sample.doc"}, Config: plugin.CodeEditorConfig{
						Language:    "yaml",
						SaveRouteID: "sample.doc.save",
						SaveMethod:  plugin.MethodPut,
						SaveParams:  map[string]string{"id": "${resource.uid}"},
					}},
					{Key: "query", Label: "Query", Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "sample.query", Method: plugin.MethodWS}, Config: plugin.QueryEditorConfig{
						InitialQuery:  "select * from ${resource.name} limit 100",
						CancelRouteID: "sample.query.cancel",
					}},
				},
			},
		}},
		Actions: []plugin.Action{{
			ID: "sample.start", Label: "Start", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "play"},
			RouteID: "sample.start", Confirm: true, ConfirmText: "Start it?",
			EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "state", Op: plugin.OpEq, Value: "stopped"}}},
		}},
		Streams: []plugin.Stream{
			{ID: "sample.shell", Kind: plugin.StreamTerminal, RouteID: "sample.shell"},
			{ID: "sample.logs", Kind: plugin.StreamLogs, RouteID: "sample.logs"},
		},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2},
			StreamIDs: []string{"sample.shell"}, Authoritative: true,
		}},
	}
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	stream := func(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }
	routes := []plugin.Route{
		{ID: "sample.shell", Method: plugin.MethodWS, Path: "/shell", Permission: "sample.shell", Risk: plugin.RiskPrivileged, AuditEvent: "sample.shell", Stream: stream},
		{ID: "sample.files.list", Method: plugin.MethodGet, Path: "/files", Permission: "sample.files.read", Risk: plugin.RiskSafe, AuditEvent: "sample.files.list", Handle: noop},
		{ID: "sample.files.read", Method: plugin.MethodGet, Path: "/files/read", Permission: "sample.files.read", Risk: plugin.RiskSafe, AuditEvent: "sample.files.read", Handle: noop},
		{ID: "sample.files.download", Method: plugin.MethodGet, Path: "/files/download", Permission: "sample.files.read", Risk: plugin.RiskSafe, AuditEvent: "sample.files.download", Handle: noop},
		{ID: "sample.files.upload", Method: plugin.MethodPost, Path: "/files/upload", Permission: "sample.files.write", Risk: plugin.RiskWrite, AuditEvent: "sample.files.upload", Handle: noop, Input: &plugin.Schema{Groups: []plugin.Group{{Name: "Upload", Fields: []plugin.Field{{Key: "files", Label: "Files", Type: plugin.FieldFile}}}}}},
		{ID: "sample.files.mkdir", Method: plugin.MethodPost, Path: "/files/mkdir", Permission: "sample.files.write", Risk: plugin.RiskWrite, AuditEvent: "sample.files.mkdir", Handle: noop},
		{ID: "sample.files.rename", Method: plugin.MethodPatch, Path: "/files/rename", Permission: "sample.files.write", Risk: plugin.RiskWrite, AuditEvent: "sample.files.rename", Handle: noop},
		{ID: "sample.files.delete", Method: plugin.MethodDelete, Path: "/files/delete", Permission: "sample.files.write", Risk: plugin.RiskDestructive, AuditEvent: "sample.files.delete", Handle: noop},
		{ID: "sample.list", Method: plugin.MethodGet, Path: "/containers", Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.list", Handle: noop},
		{ID: "sample.logs", Method: plugin.MethodWS, Path: "/logs", Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.logs", Stream: stream},
		{ID: "sample.public", Method: plugin.MethodGet, Path: "/summary", Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.public", Handle: noop},
		{ID: "sample.form", Method: plugin.MethodGet, Path: "/form", Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.form", Handle: noop},
		{ID: "sample.form.save", Method: plugin.MethodPatch, Path: "/form", Permission: "sample.write", Risk: plugin.RiskWrite, AuditEvent: "sample.form.save", Handle: noop},
		{ID: "sample.doc", Method: plugin.MethodGet, Path: "/doc", Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.doc", Handle: noop},
		{ID: "sample.doc.save", Method: plugin.MethodPut, Path: "/doc", Permission: "sample.write", Risk: plugin.RiskWrite, AuditEvent: "sample.doc.save", Handle: noop},
		{ID: "sample.query", Method: plugin.MethodWS, Path: "/query", Permission: "sample.query", Risk: plugin.RiskPrivileged, AuditEvent: "sample.query", Stream: stream},
		{ID: "sample.query.cancel", Method: plugin.MethodPost, Path: "/query/cancel", Permission: "sample.query", Risk: plugin.RiskWrite, AuditEvent: "sample.query.cancel", Handle: noop},
		{ID: "sample.start", Method: plugin.MethodPost, Path: "/start", Permission: "sample.start", Risk: plugin.RiskWrite, AuditEvent: "container.start", Handle: noop, Input: &plugin.Schema{Groups: []plugin.Group{{Name: "opts", Fields: []plugin.Field{{Key: "force", Label: "Force", Type: plugin.FieldToggle}}}}}},
	}
	return m, routes
}

func TestProjectionGolden(t *testing.T) {
	m, routes := sampleManifest()
	if err := plugin.Validate(m, routes); err != nil {
		t.Fatalf("validate: %v", err)
	}
	proj := plugin.BuildProjection(m, routeMap(routes))
	got, err := json.MarshalIndent(proj, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "projection.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("projection JSON drifted from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func routeMap(routes []plugin.Route) map[string]plugin.Route {
	out := make(map[string]plugin.Route, len(routes))
	for _, route := range routes {
		out[route.ID] = route
	}
	return out
}

func TestProjectionUnmarshalsProjectedActionConfig(t *testing.T) {
	proj := plugin.Projection{
		Name: "sample",
		Actions: []plugin.ProjectedAction{{
			ID:      "sample.edit",
			Label:   "Edit",
			RouteID: "sample.update",
			Panel:   plugin.PanelCodeEditor,
			Config: plugin.CodeEditorConfig{
				Language:    "json",
				SaveRouteID: "sample.update",
				SaveMethod:  plugin.MethodPut,
				SaveBodyKey: "document",
			},
		}},
	}
	raw, err := json.Marshal(proj)
	if err != nil {
		t.Fatal(err)
	}
	var got plugin.Projection
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	cfg, ok := got.Actions[0].Config.(plugin.CodeEditorConfig)
	if !ok {
		t.Fatalf("action config type = %T, want CodeEditorConfig", got.Actions[0].Config)
	}
	if cfg.SaveRouteID != "sample.update" || cfg.SaveMethod != plugin.MethodPut || cfg.SaveBodyKey != "document" {
		t.Fatalf("action config did not round-trip: %+v", cfg)
	}
}

// TestProjectionMatchesContract asserts the projected fields the frontend
// (projection.ts) relies on exist with the right names/values.
func TestProjectionMatchesContract(t *testing.T) {
	m, routes := sampleManifest()
	idx := map[string]plugin.Route{}
	for _, r := range routes {
		idx[r.ID] = r
	}
	proj := plugin.BuildProjection(m, idx)

	b, _ := json.Marshal(proj)
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"apiVersion", "name", "version", "title", "description", "icon", "category", "config", "capabilities", "supportedTransports", "layout", "tabs", "tree", "resources", "actions", "streams"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("projection missing required key %q", key)
		}
	}
	category, _ := raw["category"].(map[string]any)
	if category["key"] != string(plugin.CategoryShell) || category["label"] != "Shell & terminal" {
		t.Errorf("projection category unexpected: %+v", category)
	}

	actions, _ := raw["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	a := actions[0].(map[string]any)
	if a["risk"] != "write" {
		t.Errorf("action risk resolved from route: want write, got %v", a["risk"])
	}
	if a["requiresConfirm"] != true {
		t.Errorf("action requiresConfirm: want true, got %v", a["requiresConfirm"])
	}
	if a["method"] != "POST" {
		t.Errorf("action method resolved from route: want POST, got %v", a["method"])
	}
	if _, ok := a["input"]; !ok {
		t.Errorf("action input resolved from route route, missing")
	}
	ew, ok := a["enabledWhen"].(map[string]any)
	if !ok {
		t.Fatalf("action enabledWhen missing from projection: %v", a["enabledWhen"])
	}
	allOf, _ := ew["allOf"].([]any)
	if len(allOf) != 1 {
		t.Fatalf("enabledWhen.allOf: want 1 rule, got %v", ew["allOf"])
	}
	rule := allOf[0].(map[string]any)
	if rule["field"] != "state" || rule["op"] != "eq" || rule["value"] != "stopped" {
		t.Errorf("enabledWhen rule unexpected: %+v", rule)
	}
	// Server-only fields must NOT leak into the projection.
	if _, leaked := a["permission"]; leaked {
		t.Error("permission key leaked into projection")
	}
	if _, leaked := a["auditEvent"]; leaked {
		t.Error("audit event leaked into projection")
	}

	// Recording capability is projected, but the server-only stream binding is not.
	recs, ok := raw["recording"].([]any)
	if !ok || len(recs) != 1 {
		t.Fatalf("want 1 recording capability, got %v", raw["recording"])
	}
	rec := recs[0].(map[string]any)
	if rec["class"] != "terminal" {
		t.Errorf("recording class: want terminal, got %v", rec["class"])
	}
	if _, leaked := rec["streamIds"]; leaked {
		t.Error("recording streamIds (server-only) leaked into projection")
	}
	if _, leaked := rec["StreamIDs"]; leaked {
		t.Error("recording StreamIDs (server-only) leaked into projection")
	}
}
