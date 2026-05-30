package plugin_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

func testUser() models.User { return models.User{ID: "u1", Username: "ops"} }

func testSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Main", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{
			{Type: plugin.ValidatorRegex, Value: "^[a-z]+$"},
		}},
		{Key: "port", Label: "Port", Type: plugin.FieldNumber, Validators: []plugin.Validator{
			{Type: plugin.ValidatorMin, Value: 1},
			{Type: plugin.ValidatorMax, Value: 65535},
		}},
		{Key: "mode", Label: "Mode", Type: plugin.FieldSelect, Options: []plugin.Option{
			{Label: "Read", Value: "read"},
			{Label: "Write", Value: "write"},
		}},
		{Key: "features", Label: "Features", Type: plugin.FieldMultiSelect, Options: []plugin.Option{
			{Label: "Logs", Value: "logs"},
			{Label: "Metrics", Value: "metrics"},
		}},
		{Key: "advanced", Label: "Advanced", Type: plugin.FieldToggle},
		{Key: "token", Label: "Token", Type: plugin.FieldPassword, Required: true, VisibleWhen: &plugin.Condition{
			AllOf: []plugin.Rule{{Field: "advanced", Op: plugin.OpEq, Value: true}},
		}},
	}}}}
}

func TestParamListSplitsAndDropsBlanks(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), testUser(), nil,
		map[string]string{"namespace": "ns1,,ns2", "empty": ""}, nil, nil)

	got := rc.ParamList("namespace", ",")
	if len(got) != 2 || got[0] != "ns1" || got[1] != "ns2" {
		t.Errorf("ParamList split: want [ns1 ns2], got %v", got)
	}
	if rc.ParamList("empty", ",") != nil {
		t.Error("an absent/blank param should yield nil, not an empty slice")
	}
}

func TestValidateSchemaAcceptsValidJSON(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), testUser(), nil, nil, nil, []byte(`{
		"name":"alpha",
		"port":22,
		"mode":"read",
		"features":["logs"],
		"advanced":true,
		"token":"secret"
	}`))
	if err := rc.ValidateSchema(testSchema()); err != nil {
		t.Fatalf("valid schema input rejected: %v", err)
	}
}

func TestSchemaValuesWithDefaultsPreservesExplicitValues(t *testing.T) {
	schema := plugin.Schema{Groups: []plugin.Group{{Name: "Safety", Fields: []plugin.Field{
		{Key: "read_only", Label: "Read-only", Type: plugin.FieldToggle, Default: true},
		{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: 6379},
	}}}}

	defaults := schema.ValuesWithDefaults(map[string]any{"read_only": false})
	if defaults["read_only"] != false {
		t.Fatalf("explicit false was not preserved: %#v", defaults["read_only"])
	}
	if defaults["port"] != 6379 {
		t.Fatalf("missing number default was not applied: %#v", defaults["port"])
	}
}

func TestValidateSchemaRejectsMissingVisibleRequiredField(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), testUser(), nil, nil, nil, []byte(`{"name":"alpha","advanced":true}`))
	if err := rc.ValidateSchema(testSchema()); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("want token required error, got %v", err)
	}
}

func TestValidateSchemaSkipsHiddenRequiredField(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), testUser(), nil, nil, nil, []byte(`{"name":"alpha","advanced":false}`))
	if err := rc.ValidateSchema(testSchema()); err != nil {
		t.Fatalf("hidden token should not be required: %v", err)
	}
}

func TestValidateValuesUsesAmbientContext(t *testing.T) {
	schema := plugin.Schema{Groups: []plugin.Group{{Name: "Target", Fields: []plugin.Field{
		{Key: "endpoint", Label: "Endpoint", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{
			AllOf: []plugin.Rule{{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)}},
		}},
	}}}}
	directContext := map[string]any{plugin.SchemaContextTransport: string(plugin.TransportDirect)}
	if err := schema.ValidateValuesWithContext(map[string]any{}, nil, directContext); err == nil {
		t.Fatal("direct transport should require the visible endpoint")
	}
	agentContext := map[string]any{plugin.SchemaContextTransport: string(plugin.TransportAgent)}
	if err := schema.ValidateValuesWithContext(map[string]any{}, nil, agentContext); err != nil {
		t.Fatalf("agent transport should hide the endpoint: %v", err)
	}
	values := schema.VisibleValues(map[string]any{"endpoint": "127.0.0.1:2375"}, agentContext)
	if len(values) != 0 {
		t.Fatalf("hidden endpoint should be omitted from visible values, got %#v", values)
	}
}

func TestConditionRequiresAllOfAndAnyOfWhenBothAreSet(t *testing.T) {
	schema := plugin.Schema{Groups: []plugin.Group{{Name: "Main", Fields: []plugin.Field{
		{Key: "enabled", Label: "Enabled", Type: plugin.FieldToggle},
		{Key: "mode", Label: "Mode", Type: plugin.FieldText},
		{Key: "token", Label: "Token", Type: plugin.FieldPassword, Required: true, VisibleWhen: &plugin.Condition{
			AllOf: []plugin.Rule{{Field: "enabled", Op: plugin.OpEq, Value: true}},
			AnyOf: []plugin.Rule{
				{Field: "mode", Op: plugin.OpEq, Value: "password"},
				{Field: "mode", Op: plugin.OpEq, Value: "token"},
			},
		}},
	}}}}
	if err := schema.ValidateValues(map[string]any{"enabled": true, "mode": "other"}, nil); err != nil {
		t.Fatalf("field should be hidden when anyOf fails: %v", err)
	}
	if err := schema.ValidateValues(map[string]any{"enabled": true, "mode": "token"}, nil); err == nil {
		t.Fatal("field should be visible and required when allOf and anyOf pass")
	}
}

func TestValidateSchemaRejectsTypeAndOptionFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"number type", `{"name":"alpha","port":"22"}`},
		{"select option", `{"name":"alpha","mode":"admin"}`},
		{"multiselect option", `{"name":"alpha","features":["logs","admin"]}`},
		{"regex", `{"name":"Alpha"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := plugin.NewRequestContext(context.Background(), testUser(), nil, nil, nil, []byte(tt.body))
			if err := rc.ValidateSchema(testSchema()); err == nil {
				t.Fatal("invalid schema input accepted")
			}
		})
	}
}

func TestValidateSchemaAcceptsMultipartFormValues(t *testing.T) {
	rc := plugin.NewMultipartRequestContext(context.Background(), testUser(), nil, nil, nil, url.Values{
		"name":     {"alpha"},
		"port":     {"22"},
		"mode":     {"write"},
		"features": {"logs", "metrics"},
	}, nil)
	if err := rc.ValidateSchema(testSchema()); err != nil {
		t.Fatalf("valid form input rejected: %v", err)
	}
}
