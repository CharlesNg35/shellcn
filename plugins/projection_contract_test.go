package plugins

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugin/pluginux"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestAllPluginProjectionsMarshal(t *testing.T) {
	for _, p := range allTestPlugins(t) {
		name := p.Manifest().Name
		t.Run(name, func(t *testing.T) {
			proj := plugintest.Projection(t, p)
			if proj.Name != name {
				t.Fatalf("projection name = %q, want %q", proj.Name, name)
			}
			if proj.SupportedTransports == nil || proj.Capabilities == nil {
				t.Fatalf("projection %q has nil required arrays", name)
			}
			for _, action := range proj.Actions {
				if action.Method == "" || action.Risk == "" {
					t.Fatalf("action %q did not resolve route method/risk", action.ID)
				}
				if (action.Open == plugin.OpenDock || action.Open == plugin.OpenDialog) && action.Panel == "" {
					t.Fatalf("action %q opens %q without a panel", action.ID, action.Open)
				}
				if action.Risk == plugin.RiskDestructive && !action.RequiresConfirm {
					t.Fatalf("destructive action %q must require confirmation", action.ID)
				}
			}
			if findings := pluginux.Errors(pluginux.Lint(p.Manifest(), p.Routes())); len(findings) > 0 {
				for _, finding := range findings {
					t.Errorf("%s: %s", finding.Path, finding.Message)
				}
				t.Fatalf("projection %q has plugin UX errors", name)
			}
			validateProjectionPanelConfigs(t, proj)
			if _, err := json.Marshal(proj); err != nil {
				t.Fatalf("projection does not marshal: %v", err)
			}
		})
	}
}

func validateProjectionPanelConfigs(t *testing.T, proj plugin.Projection) {
	t.Helper()
	for i, panel := range proj.Tabs {
		validateProjectedPanelConfig(t, panel, proj.PanelConfigSchemas, fmt.Sprintf("tabs[%d].config", i))
	}
	for i, resource := range proj.Resources {
		for j, panel := range resource.Detail.Tabs {
			validateProjectedPanelConfig(t, panel, proj.PanelConfigSchemas, fmt.Sprintf("resources[%d].detail.tabs[%d].config", i, j))
		}
	}
	for i, action := range proj.Actions {
		if action.Panel == "" || action.Config == nil {
			continue
		}
		panel := plugin.Panel{Key: action.ID, Type: action.Panel, Config: action.Config}
		validateProjectedPanelConfig(t, panel, proj.PanelConfigSchemas, fmt.Sprintf("actions[%d].config", i))
	}
}

func validateProjectedPanelConfig(t *testing.T, panel plugin.Panel, schemas map[plugin.PanelType]plugin.PanelConfigSchema, path string) {
	t.Helper()
	if panel.Config == nil {
		return
	}
	var config map[string]any
	raw, err := json.Marshal(panel.Config)
	if err != nil {
		t.Fatalf("%s: marshal config: %v", path, err)
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("%s: unmarshal config: %v", path, err)
	}
	if err := validateConfigObject(config, schemas[panel.Type], path); err != nil {
		t.Fatal(err)
	}
	switch cfg := panel.Config.(type) {
	case plugin.DashboardConfig:
		for i, child := range cfg.Cells {
			validateProjectedPanelConfig(t, child, schemas, fmt.Sprintf("%s.cells[%d].config", path, i))
		}
	case plugin.SplitConfig:
		for i, child := range cfg.Panels {
			validateProjectedPanelConfig(t, child.Panel, schemas, fmt.Sprintf("%s.panels[%d].config", path, i))
		}
	}
}

func validateConfigObject(value map[string]any, schema plugin.PanelConfigSchema, path string) error {
	if schema.Type == "" {
		return nil
	}
	required := schema.Required
	properties := schema.Properties
	for _, key := range required {
		if value[key] == nil {
			return fmt.Errorf("%s.%s is required", path, key)
		}
	}
	for key, val := range value {
		prop, ok := properties[key]
		if !ok {
			prop, ok = properties["*"]
		}
		if !ok {
			return fmt.Errorf("%s.%s is not supported", path, key)
		}
		if err := validateConfigProperty(val, prop, path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigProperty(value any, schema plugin.PanelConfigProperty, path string) error {
	if value == nil {
		return nil
	}
	if len(schema.Enum) > 0 {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", path)
		}
		for _, option := range schema.Enum {
			if str == option {
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %v", path, schema.Enum)
	}
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", path)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s must be a number", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if schema.Items == nil {
			return nil
		}
		for i, item := range items {
			if err := validateConfigProperty(item, *schema.Items, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		if len(schema.Properties) == 0 && len(schema.Required) == 0 {
			return nil
		}
		return validateConfigObject(obj, plugin.PanelConfigSchema{Type: "object", Properties: schema.Properties, Required: schema.Required}, path)
	}
	return nil
}
