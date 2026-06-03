package plugin_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestConfigRoundTrip(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "demo",
		Tabs: []plugin.Panel{
			{Key: "data", Type: plugin.PanelTable, Config: plugin.TableConfig{Editable: true, RowKey: []string{"id"}}},
			{Key: "logs", Type: plugin.PanelLogStream},
		},
		Actions: []plugin.Action{
			{ID: "edit", RouteID: "demo.edit", Open: plugin.OpenDock, Panel: plugin.PanelCodeEditor, Config: plugin.CodeEditorConfig{Language: "sql"}},
		},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got plugin.Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := got.Tabs[0].Config.(plugin.TableConfig); !ok {
		t.Fatalf("table config lost its type: %T", got.Tabs[0].Config)
	}
	if got.Tabs[1].Config != nil {
		t.Fatalf("configless panel gained a config: %T", got.Tabs[1].Config)
	}
	if _, ok := got.Actions[0].Config.(plugin.CodeEditorConfig); !ok {
		t.Fatalf("action config lost its type: %T", got.Actions[0].Config)
	}

	again, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(data, again) {
		t.Fatalf("round-trip not byte-identical:\n %s\n %s", data, again)
	}
}
