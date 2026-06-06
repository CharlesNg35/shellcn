package plugintest

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ValidateProjectionPanelConfigs checks every projected panel config against the
// SDK panel config schemas used by the browser renderer.
func ValidateProjectionPanelConfigs(t testing.TB, proj plugin.Projection) {
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

func validateProjectedPanelConfig(t testing.TB, panel plugin.Panel, schemas map[plugin.PanelType]plugin.PanelConfigSchema, path string) {
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
	for _, key := range schema.Required {
		if value[key] == nil {
			return fmt.Errorf("%s.%s is required", path, key)
		}
	}
	for key, val := range value {
		prop, ok := schema.Properties[key]
		if !ok {
			prop, ok = schema.Properties["*"]
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
		return validateConfigObject(obj, plugin.PanelConfigSchema{
			Type:       "object",
			Properties: schema.Properties,
			Required:   schema.Required,
		}, path)
	}
	return nil
}
