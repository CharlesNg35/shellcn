package plugin_test

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestPanelConfigSchemasExposeMetricItemFields(t *testing.T) {
	schema := plugin.PanelConfigSchemas()[plugin.PanelMetrics]
	stats := schema.Properties["stats"].Items
	if stats == nil {
		t.Fatal("metrics stats schema has no item schema")
	}
	if stats.Properties["key"].Type != "string" || stats.Properties["label"].Type != "string" || stats.Properties["unit"].Type != "string" {
		t.Fatalf("metric stat schema properties = %#v, want key/label/unit strings", stats.Properties)
	}

	gauges := schema.Properties["gauges"].Items
	if gauges == nil {
		t.Fatal("metrics gauges schema has no item schema")
	}
	if gauges.Properties["max"].Type != "number" {
		t.Fatalf("metric gauge max schema = %#v, want number", gauges.Properties["max"])
	}
}
