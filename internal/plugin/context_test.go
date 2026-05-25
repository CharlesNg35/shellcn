package plugin_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
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
