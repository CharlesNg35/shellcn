package s3

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func TestManifestExposesStorageAffordances(t *testing.T) {
	m := New().Manifest()
	if len(m.Tabs) != 2 {
		t.Fatalf("tabs = %d, want files and buckets", len(m.Tabs))
	}
	files, ok := m.Tabs[0].Config.(plugin.FileBrowserConfig)
	if !ok || files.Routes.Move == "" || files.Routes.Copy == "" || files.Routes.Archive == "" {
		t.Fatalf("s3 file browser missing object operation affordances: %#v", m.Tabs[0].Config)
	}
	buckets, ok := m.Tabs[1].Config.(plugin.TableConfig)
	if !ok || buckets.EmptyText == "" || buckets.RowClick != plugin.RowClickDetail {
		t.Fatalf("bucket table missing tailored empty/detail affordances: %#v", m.Tabs[1].Config)
	}
	actions := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actions[action.ID] = action
	}
	versioning := actions["s3.bucket.versioning.set"]
	if !versioning.Confirm || versioning.ConfirmText == "" {
		t.Fatalf("versioning action should require confirmation: %+v", versioning)
	}
}

func TestBucketFieldDeclaresPortableNameValidation(t *testing.T) {
	m := New().Manifest()
	for _, group := range m.Config.Groups {
		for _, field := range group.Fields {
			if field.Key == "bucket" {
				if len(field.Validators) == 0 || field.Validators[0].Type != plugin.ValidatorRegex {
					t.Fatalf("bucket field missing regex validator: %+v", field)
				}
				return
			}
		}
	}
	t.Fatal("missing bucket field")
}

func TestConfigUsesObjectStoreSpecificControls(t *testing.T) {
	m := New().Manifest()
	region := requireConfigField(t, m.Config, "region")
	if region.Type != plugin.FieldAutocomplete || len(region.Options) < 10 {
		t.Fatalf("region should be autocomplete with AWS region suggestions: %+v", region)
	}
	prefix := requireConfigField(t, m.Config, "prefix")
	if prefix.Type != plugin.FieldAutocomplete {
		t.Fatalf("root prefix should be autocomplete/free text, got %+v", prefix)
	}
	endpoint := requireConfigField(t, m.Config, "endpoint")
	if endpoint.Type != plugin.FieldURL || endpoint.Help == "" {
		t.Fatalf("endpoint should use URL input with help: %+v", endpoint)
	}
	pathStyle := requireConfigField(t, m.Config, "path_style")
	if pathStyle.Type != plugin.FieldToggle || pathStyle.Help == "" {
		t.Fatalf("path style should be an explained toggle: %+v", pathStyle)
	}
}

func requireConfigField(t *testing.T, schema plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("missing config field %q", key)
	return plugin.Field{}
}
