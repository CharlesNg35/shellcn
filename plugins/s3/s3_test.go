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
	if !ok || files.MoveRouteID == "" || files.CopyRouteID == "" || files.ArchiveRouteID == "" {
		t.Fatalf("s3 file browser missing object operation routes: %#v", m.Tabs[0].Config)
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
