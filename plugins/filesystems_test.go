package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestFilesystemPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"ftp", "ftps", "webdav", "smb", "s3"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategoryFiles {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategoryFiles)
		}
		// Every filesystem plugin leads with a file browser. Pure transfer
		// protocols expose exactly that; object stores add bucket
		// management tabs alongside it.
		objectStore := name == "s3"
		if proj.Tabs[0].Type != plugin.PanelFileBrowser ||
			(objectStore && len(proj.Tabs) < 1) || (!objectStore && len(proj.Tabs) != 1) {
			t.Fatalf("%s should lead with a file browser tab: %+v", name, proj.Tabs)
		}
		fb, ok := proj.Tabs[0].Config.(plugin.FileBrowserConfig)
		if !ok || fb.ReadRouteID == "" || fb.DownloadRouteID == "" || fb.UploadRouteID == "" ||
			fb.MkdirRouteID == "" || fb.RenameRouteID == "" || fb.DeleteRouteID == "" {
			t.Fatalf("%s files config missing route ids: %#v", name, proj.Tabs[0].Config)
		}
	}
}

func TestPluginConfigDefaultsSatisfyNumericValidators(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, p := range reg.All() {
		m := p.Manifest()
		for _, group := range m.Config.Groups {
			for _, field := range group.Fields {
				if field.Default == nil {
					continue
				}
				value, ok := numericValue(field.Default)
				if !ok {
					continue
				}
				for _, validator := range field.Validators {
					limit, ok := numericValue(validator.Value)
					if !ok {
						continue
					}
					switch validator.Type {
					case plugin.ValidatorMin:
						if value < limit {
							t.Fatalf("%s config field %q default %v is below min %v", m.Name, field.Key, field.Default, validator.Value)
						}
					case plugin.ValidatorMax:
						if value > limit {
							t.Fatalf("%s config field %q default %v is above max %v", m.Name, field.Key, field.Default, validator.Value)
						}
					}
				}
			}
		}
	}
}

func TestPasswordAndStoredCredentialAreMutuallyExclusiveByDefault(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, p := range reg.All() {
		m := p.Manifest()
		fields := fieldMap(m.Config)
		if !fields["password"] || !fields["credential_id"] {
			continue
		}
		visible := m.Config.VisibleValues(m.Config.ValuesWithDefaults(map[string]any{}), nil)
		if _, passwordVisible := visible["password"]; passwordVisible {
			if _, credentialVisible := visible["credential_id"]; credentialVisible {
				t.Fatalf("%s shows password and credential selector together by default", m.Name)
			}
		}
	}
}

func TestSharedBasicAuthCredentialCompatibility(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"ftp", "ftps", "webdav", "smb"} {
		if !reg.CredentialKindSupportsProtocol(plugin.CredentialBasicAuth, name) {
			t.Fatalf("basic auth credential should support %s", name)
		}
	}
	for _, name := range []string{"s3"} {
		if !reg.CredentialKindSupportsProtocol(plugin.CredentialCloudAccessKey, name) {
			t.Fatalf("cloud access key credential should support %s", name)
		}
	}
}

func TestFilesystemAuthSchemasAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"ftp", "ftps", "webdav", "smb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		fields := fieldMap(m.Config)
		for _, key := range []string{"machine_name", "uid", "gid", "export_path"} {
			if fields[key] {
				t.Fatalf("%s should not include NFS field %q", name, key)
			}
		}
		if !fields["credential_id"] || !fields["username"] || !fields["password"] {
			t.Fatalf("%s should expose username/password and stored credential fields: %+v", name, fields)
		}
	}

	for _, name := range []string{"s3"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		fields := fieldMap(m.Config)
		for _, key := range []string{"username", "password", "machine_name", "uid", "gid"} {
			if fields[key] {
				t.Fatalf("%s should not include non-S3 auth field %q", name, key)
			}
		}
		for _, key := range []string{"access_key_id", "secret_access_key", "credential_id", "bucket", "region"} {
			if !fields[key] {
				t.Fatalf("%s should include S3-compatible field %q", name, key)
			}
		}
	}
}

func TestFilesystemAuthVisibleValuesAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"ftp", "ftps", "webdav", "smb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		password := visibleFilesystemFields(m.Config, map[string]any{"auth": "password"})
		requireFilesystemVisible(t, name, password, "username", "password")
		requireFilesystemHidden(t, name, password, "credential_id", "machine_name", "uid", "gid")

		credential := visibleFilesystemFields(m.Config, map[string]any{"auth": "credential"})
		requireFilesystemVisible(t, name, credential, "credential_id")
		requireFilesystemHidden(t, name, credential, "username", "password", "machine_name", "uid", "gid")
	}

	for _, name := range []string{"s3"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		accessKey := visibleFilesystemFields(m.Config, map[string]any{"auth": "access_key"})
		requireFilesystemVisible(t, name, accessKey, "access_key_id", "secret_access_key", "session_token")
		requireFilesystemHidden(t, name, accessKey, "credential_id", "username", "password", "machine_name", "uid", "gid")

		credential := visibleFilesystemFields(m.Config, map[string]any{"auth": "credential"})
		requireFilesystemVisible(t, name, credential, "credential_id", "session_token")
		requireFilesystemHidden(t, name, credential, "access_key_id", "secret_access_key", "username", "password", "machine_name", "uid", "gid")
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	fields := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			fields[field.Key] = true
		}
	}
	return fields
}

func visibleFilesystemFields(schema plugin.Schema, overrides map[string]any) map[string]bool {
	values := schema.Defaults()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if _, ok := values[field.Key]; !ok {
				values[field.Key] = blankFilesystemFieldValue(field.Type)
			}
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	visible := schema.VisibleValues(values, nil)
	out := map[string]bool{}
	for key := range visible {
		out[key] = true
	}
	return out
}

func blankFilesystemFieldValue(t plugin.FieldType) any {
	switch t {
	case plugin.FieldNumber:
		return float64(0)
	case plugin.FieldToggle:
		return false
	case plugin.FieldMultiSelect:
		return []any{}
	default:
		return ""
	}
}

func requireFilesystemVisible(t *testing.T, pluginName string, visible map[string]bool, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if !visible[key] {
			t.Fatalf("%s should show %q for this auth mode; visible=%v", pluginName, key, visible)
		}
	}
}

func requireFilesystemHidden(t *testing.T, pluginName string, visible map[string]bool, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if visible[key] {
			t.Fatalf("%s should hide %q for this auth mode; visible=%v", pluginName, key, visible)
		}
	}
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
