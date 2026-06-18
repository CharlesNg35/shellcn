package grpcplugin_test

import (
	"encoding/json"
	"testing"

	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// TestEncodeDecodePreservesProjection is the golden test: a manifest round-tripped
// through the wire codec must project byte-identically to the in-process manifest.
func TestEncodeDecodePreservesProjection(t *testing.T) {
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "demo",
		Title:               "Demo",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{{
			Key: "data", Type: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "demo.list"},
			Config: plugin.TableConfig{
				Columns: []plugin.Column{
					{Key: "id", Label: "ID", ReadOnly: true},
					{Key: "name", Label: "Name", Editable: true, Editor: plugin.ColumnEditorText},
				},
				Editable: true,
				RowKey:   []string{"id"},
				Update:   &plugin.DataSource{RouteID: "demo.row.update", Method: plugin.MethodPost},
			},
		}},
		Actions: []plugin.Action{
			{ID: "edit", RouteID: "demo.edit", Open: plugin.OpenDock, Panel: plugin.PanelCodeEditor, Config: plugin.CodeEditorConfig{Language: "sql"}},
		},
	}
	routes := []plugin.Route{
		{ID: "demo.list", Method: plugin.MethodGet, Path: "/list", Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.list"},
		{ID: "demo.row.update", Method: plugin.MethodPost, Path: "/row/update", Permission: "demo.write", Risk: plugin.RiskWrite, AuditEvent: "demo.row.update"},
		{ID: "demo.edit", Method: plugin.MethodWS, Path: "/edit", Permission: "demo.edit", Risk: plugin.RiskPrivileged, AuditEvent: "demo.edit"},
	}

	data, err := grpcplugin.EncodeManifest(m, routes)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	gotM, gotR, err := grpcplugin.DecodeManifest(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := plugin.BuildProjection(m, indexRoutes(routes))
	got := plugin.BuildProjection(gotM, indexRoutes(gotR))
	if mustJSON(t, got) != mustJSON(t, want) {
		t.Fatalf("projection changed across encode/decode:\n got %s\nwant %s", mustJSON(t, got), mustJSON(t, want))
	}
}

func indexRoutes(rs []plugin.Route) map[string]plugin.Route {
	out := make(map[string]plugin.Route, len(rs))
	for _, r := range rs {
		out[r.ID] = r
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
