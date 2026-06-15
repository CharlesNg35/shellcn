package plugin_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestPanelConfigSchemasCoverConfigJSONFields(t *testing.T) {
	cases := []struct {
		panel plugin.PanelType
		typ   reflect.Type
	}{
		{plugin.PanelTable, reflect.TypeOf(plugin.TableConfig{})},
		{plugin.PanelFileBrowser, reflect.TypeOf(plugin.FileBrowserConfig{})},
		{plugin.PanelForm, reflect.TypeOf(plugin.FormPanelConfig{})},
		{plugin.PanelDashboard, reflect.TypeOf(plugin.DashboardConfig{})},
		{plugin.PanelMetrics, reflect.TypeOf(plugin.MetricsConfig{})},
		{plugin.PanelGraph, reflect.TypeOf(plugin.GraphConfig{})},
		{plugin.PanelTrace, reflect.TypeOf(plugin.TraceConfig{})},
		{plugin.PanelKV, reflect.TypeOf(plugin.KVConfig{})},
		{plugin.PanelTerminal, reflect.TypeOf(plugin.TerminalConfig{})},
		{plugin.PanelTerminalGrid, reflect.TypeOf(plugin.TerminalGridConfig{})},
		{plugin.PanelCodeEditor, reflect.TypeOf(plugin.CodeEditorConfig{})},
		{plugin.PanelDiff, reflect.TypeOf(plugin.DiffConfig{})},
		{plugin.PanelQueryEditor, reflect.TypeOf(plugin.QueryEditorConfig{})},
		{plugin.PanelHTTPClient, reflect.TypeOf(plugin.HTTPClientConfig{})},
		{plugin.PanelRemoteDesktop, reflect.TypeOf(plugin.RemoteDesktopConfig{})},
		{plugin.PanelObjectDetail, reflect.TypeOf(plugin.ObjectDetailConfig{})},
		{plugin.PanelTimeline, reflect.TypeOf(plugin.TimelineConfig{})},
		{plugin.PanelTaskProgress, reflect.TypeOf(plugin.TaskProgressConfig{})},
		{plugin.PanelSplit, reflect.TypeOf(plugin.SplitConfig{})},
		{plugin.PanelCanvas, reflect.TypeOf(plugin.CanvasConfig{})},
		{plugin.PanelWasm, reflect.TypeOf(plugin.WasmConfig{})},
	}

	schemas := plugin.PanelConfigSchemas()
	for _, tc := range cases {
		t.Run(string(tc.panel), func(t *testing.T) {
			schema, ok := schemas[tc.panel]
			if !ok {
				t.Fatalf("missing schema for %q", tc.panel)
			}
			assertSchemaMatchesStruct(t, string(tc.panel), tc.typ, schema.Properties)
		})
	}
}

func TestPanelConfigSchemasCoverConfiglessPanelTypes(t *testing.T) {
	schemas := plugin.PanelConfigSchemas()
	for _, panel := range []plugin.PanelType{
		plugin.PanelDocument,
		plugin.PanelLogStream,
		plugin.PanelEnroll,
	} {
		t.Run(string(panel), func(t *testing.T) {
			schema, ok := schemas[panel]
			if !ok {
				t.Fatalf("missing schema for %q", panel)
			}
			if schema.Type != "object" {
				t.Fatalf("schema type = %q, want object", schema.Type)
			}
			if len(schema.Properties) != 0 {
				t.Fatalf("schema properties = %#v, want closed empty object", schema.Properties)
			}
		})
	}
}

func TestNestedPanelConfigSchemasCoverSDKTypes(t *testing.T) {
	schemas := plugin.PanelConfigSchemas()
	assertArrayItemSchemaMatchesStruct(t, "metrics.stats", plugin.MetricStat{}, schemas[plugin.PanelMetrics].Properties["stats"])
	assertArrayItemSchemaMatchesStruct(t, "metrics.gauges", plugin.MetricGauge{}, schemas[plugin.PanelMetrics].Properties["gauges"])
	assertArrayItemSchemaMatchesStruct(t, "metrics.series", plugin.MetricSeries{}, schemas[plugin.PanelMetrics].Properties["series"])
	assertArrayItemSchemaMatchesStruct(t, "http_client.defaultHeaders", plugin.HeaderDefault{}, schemas[plugin.PanelHTTPClient].Properties["defaultHeaders"])
	assertArrayItemSchemaMatchesStruct(t, "dashboard.cells", plugin.Panel{}, schemas[plugin.PanelDashboard].Properties["cells"])
	assertObjectSchemaMatchesStruct(t, "file_browser.routes", plugin.FileBrowserRoutes{}, schemas[plugin.PanelFileBrowser].Properties["routes"])
	assertObjectSchemaMatchesStruct(t, "file_browser.upload", plugin.FileUploadConfig{}, schemas[plugin.PanelFileBrowser].Properties["upload"])
	assertArrayItemSchemaMatchesStruct(t, "split.panels", plugin.SplitPanel{}, schemas[plugin.PanelSplit].Properties["panels"])
	assertArrayItemSchemaMatchesStruct(t, "object_detail.sections", plugin.ObjectDetailSection{}, schemas[plugin.PanelObjectDetail].Properties["sections"])
	assertArrayItemSchemaMatchesStruct(t, "object_detail.sections.fields", plugin.ObjectDetailField{}, schemas[plugin.PanelObjectDetail].Properties["sections"].Items.Properties["fields"])
	assertArrayItemSchemaMatchesStruct(t, "wasm.assets", plugin.WasmAsset{}, schemas[plugin.PanelWasm].Properties["assets"])
	assertArrayItemSchemaMatchesStruct(t, "wasm.bridge.routes", plugin.WasmBridgeRoute{}, schemas[plugin.PanelWasm].Properties["bridge"].Properties["routes"])
	assertArrayItemSchemaMatchesStruct(t, "wasm.bridge.streams", plugin.WasmBridgeStream{}, schemas[plugin.PanelWasm].Properties["bridge"].Properties["streams"])
}

func assertObjectSchemaMatchesStruct(t *testing.T, path string, sample any, schema plugin.PanelConfigProperty) {
	t.Helper()
	if schema.Type != "object" || len(schema.Properties) == 0 {
		t.Fatalf("%s schema = %#v, want object schema", path, schema)
	}
	assertSchemaMatchesStruct(t, path, reflect.TypeOf(sample), schema.Properties)
}

func assertArrayItemSchemaMatchesStruct(t *testing.T, path string, sample any, schema plugin.PanelConfigProperty) {
	t.Helper()
	if schema.Type != "array" || schema.Items == nil {
		t.Fatalf("%s schema = %#v, want array with item schema", path, schema)
	}
	if schema.Items.Type != "object" || len(schema.Items.Properties) == 0 {
		t.Fatalf("%s item schema = %#v, want closed object schema", path, schema.Items)
	}
	assertSchemaMatchesStruct(t, path+"[]", reflect.TypeOf(sample), schema.Items.Properties)
}

func assertSchemaMatchesStruct(t *testing.T, path string, typ reflect.Type, properties map[string]plugin.PanelConfigProperty) {
	t.Helper()
	want := jsonFieldNames(typ)
	for name := range want {
		if _, ok := properties[name]; !ok {
			t.Fatalf("%s schema missing JSON field %q", path, name)
		}
	}
	for name := range properties {
		if name == "*" {
			continue
		}
		if _, ok := want[name]; !ok {
			t.Fatalf("%s schema has unknown field %q", path, name)
		}
	}
}

func jsonFieldNames(typ reflect.Type) map[string]struct{} {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	out := map[string]struct{}{}
	if typ.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Anonymous {
			for name := range jsonFieldNames(field.Type) {
				out[name] = struct{}{}
			}
			continue
		}
		name := jsonName(field)
		if name == "" || name == "-" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func jsonName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}
