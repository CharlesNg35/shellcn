package sftp_test

import (
	"testing"

	"github.com/charlesng35/shellcn/plugins/sftp"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestManifestValidates(t *testing.T) {
	plugintest.ValidatePlugin(t, sftp.New())
}

func TestManifestIsFileOnly(t *testing.T) {
	m := sftp.New().Manifest()
	if m.Icon.Value == "folder" {
		t.Fatal("sftp connection icon must not reuse the folder glyph")
	}
	if len(m.Tabs) != 1 || m.Tabs[0].Type != plugin.PanelFileBrowser {
		t.Fatalf("sftp should expose only file_browser tab: %+v", m.Tabs)
	}
	cfg, ok := m.Tabs[0].Config.(plugin.FileBrowserConfig)
	if !ok || cfg.Routes.Move == "" || cfg.Routes.Copy == "" ||
		cfg.Routes.Chmod == "" || cfg.Routes.Archive == "" {
		t.Fatalf("sftp file browser missing bulk affordances: %#v", m.Tabs[0].Config)
	}
	if len(m.Streams) != 0 || len(m.Recording) != 0 {
		t.Fatalf("sftp must not expose streams/recording: streams=%+v recording=%+v", m.Streams, m.Recording)
	}
	for _, route := range sftp.New().Routes() {
		if route.ID == "sftp.shell" {
			t.Fatal("file-only sftp plugin exposed shell route")
		}
	}
}

func TestManifestSurfacesHostKeyVerification(t *testing.T) {
	m := sftp.New().Manifest()
	policy := requireField(t, m.Config, "host_key_verification")
	if policy.Type != plugin.FieldSelect || policy.Default != "pinned" || len(policy.Options) != 2 {
		t.Fatalf("host key verification should be an explicit select: %+v", policy)
	}
	if policy.Options[0].Value != "pinned" || policy.Options[1].Value != "insecure" {
		t.Fatalf("host key verification options should prefer pinned verification: %+v", policy.Options)
	}
	hostKey := requireField(t, m.Config, "host_key")
	if hostKey.Type != plugin.FieldTextarea || hostKey.Secret || hostKey.Help == "" || hostKey.VisibleWhen == nil {
		t.Fatalf("host_key field should be a conditional visible textarea with help: %+v", hostKey)
	}
}

func requireField(t *testing.T, schema plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("missing field %q", key)
	return plugin.Field{}
}
