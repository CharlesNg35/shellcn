package plugintest

import (
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestValidateProjectionPanelConfigsAcceptsNestedPanels(t *testing.T) {
	proj := plugin.Projection{
		PanelConfigSchemas: plugin.PanelConfigSchemas(),
		Tabs: []plugin.Panel{{
			Key:  "overview",
			Type: plugin.PanelDashboard,
			Config: plugin.DashboardConfig{Cells: []plugin.Panel{{
				Key:  "metrics",
				Type: plugin.PanelMetrics,
				Config: plugin.MetricsConfig{
					Stats: []plugin.MetricStat{{Key: "cpu", Label: "CPU"}},
				},
			}}},
		}},
	}

	ValidateProjectionPanelConfigs(t, proj)
}

func TestValidateConfigObjectRejectsUnknownNestedFields(t *testing.T) {
	err := validateConfigObject(
		map[string]any{"stats": []any{map[string]any{"label": "CPU", "unknown": true}}},
		plugin.PanelConfigSchemas()[plugin.PanelMetrics],
		"config",
	)
	if err == nil || !strings.Contains(err.Error(), "config.stats[0].unknown is not supported") {
		t.Fatalf("error = %v, want unsupported nested field", err)
	}
}

func TestValidateConfigObjectRejectsClosedEmptyConfig(t *testing.T) {
	err := validateConfigObject(
		map[string]any{"_recording": map[string]any{}},
		plugin.PanelConfigSchemas()[plugin.PanelLogStream],
		"config",
	)
	if err == nil || !strings.Contains(err.Error(), "config._recording is not supported") {
		t.Fatalf("error = %v, want unsupported field", err)
	}
}
