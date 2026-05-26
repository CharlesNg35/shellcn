package plugins

import (
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestFilesystemPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"ftp", "ftps", "webdav", "smb", "nfs"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategoryFiles {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategoryFiles)
		}
		if len(proj.Tabs) != 1 || proj.Tabs[0].Panel != plugin.PanelFileBrowser {
			t.Fatalf("%s should expose one file browser tab: %+v", name, proj.Tabs)
		}
		for _, key := range []string{"readRouteId", "downloadRouteId", "uploadRouteId", "mkdirRouteId", "renameRouteId", "deleteRouteId"} {
			if proj.Tabs[0].Config[key] == "" {
				t.Fatalf("%s files config missing %s", name, key)
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
	if reg.CredentialKindSupportsProtocol(plugin.CredentialBasicAuth, "nfs") {
		t.Fatal("nfs should not claim basic auth credential support")
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

	nfsManifest, ok := reg.Manifest("nfs")
	if !ok {
		t.Fatal("nfs plugin was not registered")
	}
	nfsFields := fieldMap(nfsManifest.Config)
	for _, key := range []string{"auth", "credential_id", "username", "password"} {
		if nfsFields[key] {
			t.Fatalf("nfs should not include password auth field %q", key)
		}
	}
	for _, key := range []string{"machine_name", "uid", "gid", "export_path"} {
		if !nfsFields[key] {
			t.Fatalf("nfs should include AUTH_SYS/export field %q", key)
		}
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
